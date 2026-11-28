'use strict';
/**
 * extension.js - Lean VS Code / Open VSX integration for the Apache-2.0 Nomos Go execution substrate.
 *
 * Strictly read-only on Day 0: never writes repository files or downloads binaries without explicit
 * user invocation. Provides status bar AI Hangover Score, sub-15ms on-save fast-file AST diagnostics,
 * full 42-gate SARIF verification, and stealth `nomos init --local` setup.
 */
const { execFile } = require('node:child_process');
const path = require('node:path');
const vscode = require('vscode');
const pkg = require('./package.json');
const { downloadVerifiedBinary, resolveLocalBinary } = require('./lib/binary');
const {
	formatStatusBarText,
	parseFastFileRadarOutput,
	parseSarifDiagnostics,
	parseXrayReport
} = require('./lib/parsers');

const SUPPORTED_EXTS = new Set([
	'.go', '.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs', '.svelte',
	'.py', '.rs', '.c', '.cc', '.cpp', '.h', '.hpp', '.java', '.kt', '.rb', '.cs'
]);

function getWorkspaceRoot() {
	const folder = vscode.workspace.workspaceFolders?.[0];
	return folder ? folder.uri.fsPath : '';
}

function getConfig() {
	const cfg = vscode.workspace.getConfiguration('nomos');
	return {
		binaryPath: String(cfg.get('binaryPath') || '').trim(),
		verifyOnSave: cfg.get('verifyOnSave', true) !== false
	};
}

function execNomos(binPath, args, cwd) {
	return new Promise((resolve) => {
		execFile(
			binPath,
			args,
			{ cwd, maxBuffer: 8 * 1024 * 1024, env: { ...process.env, NOMOS_NO_UPDATE_CHECK: '1' } },
			(err, stdout, stderr) => {
				const code = err && typeof err.code === 'number' ? err.code : err ? 1 : 0;
				resolve({ code, stdout: String(stdout || ''), stderr: String(stderr || '') });
			}
		);
	});
}

function toVsSeverity(sev) {
	if (sev === 'error') return vscode.DiagnosticSeverity.Error;
	if (sev === 'info') return vscode.DiagnosticSeverity.Information;
	return vscode.DiagnosticSeverity.Warning;
}

function publishDiagnostics(collection, rootDir, items) {
	const grouped = new Map();
	for (const item of items) {
		const absPath = path.isAbsolute(item.file) ? item.file : path.join(rootDir, item.file);
		const lineIdx = Math.max(0, (item.line || 1) - 1);
		const colIdx = Math.max(0, (item.col || 1) - 1);
		const range = new vscode.Range(lineIdx, colIdx, lineIdx, colIdx + 1);
		const diag = new vscode.Diagnostic(range, item.message, toVsSeverity(item.severity));
		diag.source = 'Nomos';
		diag.code = item.ruleId || 'nomos';
		if (!grouped.has(absPath)) grouped.set(absPath, []);
		grouped.get(absPath).push(diag);
	}
	for (const [absPath, diags] of grouped.entries()) {
		collection.set(vscode.Uri.file(absPath), diags);
	}
}

async function ensureBinaryForCommand(context, output) {
	const { binaryPath } = getConfig();
	const storageDir = context.globalStorageUri.fsPath;
	const local = resolveLocalBinary({ binaryPath, storageDir, version: pkg.version });
	if (local) return local;

	if (binaryPath) {
		vscode.window.showErrorMessage(`Nomos: configured nomos.binaryPath was not found or is not executable: ${binaryPath}`);
		return '';
	}
	const choice = await vscode.window.showInformationMessage(
		`Nomos CLI (v${pkg.version}) is not on PATH. Download the SHA-256 verified Apache-2.0 static binary from GitHub Releases?`,
		'Download Verified Binary',
		'Cancel'
	);
	if (choice !== 'Download Verified Binary') return '';

	return vscode.window.withProgress(
		{ location: vscode.ProgressLocation.Notification, title: `Downloading Nomos v${pkg.version} (SHA-256 verified)...` },
		async () => {
			try {
				const downloaded = await downloadVerifiedBinary(pkg.version, storageDir);
				output.appendLine(`[nomos] Verified binary installed to ${downloaded}`);
				return downloaded;
			} catch (err) {
				const msg = err instanceof Error ? err.message : String(err);
				output.appendLine(`[nomos] Download failed: ${msg}`);
				vscode.window.showErrorMessage(`Nomos download failed: ${msg}`);
				return '';
			}
		}
	);
}

async function executeXrayCommand(context, ui) {
	const root = getWorkspaceRoot();
	if (!root) return vscode.window.showWarningMessage('Nomos: Open a workspace folder first.');
	const bin = await ensureBinaryForCommand(context, ui.output);
	if (!bin) return;
	ui.statusBar.text = '$(sync~spin) Nomos X-Ray...';
	const res = await execNomos(bin, ['xray', '.', '--json'], root);
	if (res.code !== 0 || !res.stdout.trim()) {
		ui.statusBar.text = formatStatusBarText(null);
		ui.output.appendLine(res.stderr || res.stdout || 'nomos xray failed');
		ui.output.show(true);
		return;
	}
	const report = parseXrayReport(res.stdout);
	ui.statusBar.text = formatStatusBarText(report);
	ui.statusBar.tooltip = `Nomos X-Ray: Grade ${report.grade} (${report.hangoverScore}/100) · ${report.filesScanned} files (${report.totalLoc} LOC) in ${report.durationMs}ms`;
	ui.output.clear();
	ui.output.appendLine(`Nomos Codebase X-Ray · Grade ${report.grade} (AI Hangover Score: ${report.hangoverScore}/100)`);
	ui.output.appendLine(`Scanned ${report.filesScanned} files (${report.totalLoc} LOC) in ${report.durationMs}ms · initialized=${report.initialized}`);
	ui.diagnostics.clear();
	const hotspotDiags = report.hotspots.map((h) => ({
		file: h.file,
		line: 1,
		col: 1,
		severity: h.cds > 45 ? 'warning' : 'info',
		ruleId: 'nomos-xray',
		message: `X-Ray hotspot (${h.loc} LOC, CDS ${h.cds}): ${h.reasons.join('; ') || 'structural debt'}`
	}));
	publishDiagnostics(ui.diagnostics, root, hotspotDiags);
	ui.output.show(true);
}

async function executeVerifyCommand(context, ui) {
	const root = getWorkspaceRoot();
	if (!root) return vscode.window.showWarningMessage('Nomos: Open a workspace folder first.');
	const bin = await ensureBinaryForCommand(context, ui.output);
	if (!bin) return;
	const res = await execNomos(bin, ['verify', '--sarif'], root);
	ui.diagnostics.clear();
	const parsed = parseSarifDiagnostics(res.stdout);
	publishDiagnostics(ui.diagnostics, root, parsed);
	ui.output.clear();
	ui.output.appendLine(`Nomos Verify (42 AST Gates): exit=${res.code} · findings=${parsed.length}`);
	if (res.stderr.trim()) ui.output.appendLine(res.stderr.trim());
	if (res.code === 0 && parsed.length === 0) {
		vscode.window.showInformationMessage('Nomos Verify: All 42 quality gates passed.');
	} else {
		ui.output.show(true);
	}
}

async function executeInitCommand(context, ui, localStealth) {
	const root = getWorkspaceRoot();
	if (!root) return vscode.window.showWarningMessage('Nomos: Open a workspace folder first.');
	const bin = await ensureBinaryForCommand(context, ui.output);
	if (!bin) return;
	const args = localStealth ? ['init', '--local'] : ['init'];
	const res = await execNomos(bin, args, root);
	ui.output.clear();
	ui.output.appendLine(res.stdout || res.stderr);
	ui.output.show(true);
	if (res.code === 0) {
		vscode.window.showInformationMessage(
			localStealth ? 'Nomos Branch Guard initialized in stealth mode (.git/info/exclude).' : 'Nomos Branch Guard initialized (nomos.yaml).'
		);
	}
}

async function executeInitVSCodeHooksCommand(context, ui) {
	const root = getWorkspaceRoot();
	if (!root) return vscode.window.showWarningMessage('Nomos: Open a workspace folder first.');
	const bin = await ensureBinaryForCommand(context, ui.output);
	if (!bin) return;
	const res = await execNomos(bin, ['init', '--agents', 'vscode'], root);
	ui.output.clear();
	ui.output.appendLine(res.stdout || res.stderr);
	ui.output.show(true);
	if (res.code === 0) {
		vscode.window.showInformationMessage('Nomos VS Code agent hooks wired (.github/hooks/nomos.json).');
	}
}

function registerMcpProvider(context) {
	const lm = vscode.lm;
	if (!lm || typeof lm.registerMcpServerDefinitionProvider !== 'function' || typeof vscode.McpStdioServerDefinition !== 'function') {
		return null;
	}
	return lm.registerMcpServerDefinitionProvider('nomos.mcp', {
		provideMcpServerDefinitions() {
			const { binaryPath } = getConfig();
			const bin = resolveLocalBinary({ binaryPath, storageDir: context.globalStorageUri.fsPath, version: pkg.version });
			if (!bin) return [];
			const def = new vscode.McpStdioServerDefinition('Nomos', bin, ['mcp'], pkg.version);
			const root = getWorkspaceRoot();
			if (root && vscode.Uri && typeof vscode.Uri.file === 'function') {
				def.cwd = vscode.Uri.file(root);
			}
			return [def];
		}
	});
}

async function executeCopyMarkdownCommand(context, ui) {
	const root = getWorkspaceRoot();
	if (!root) return vscode.window.showWarningMessage('Nomos: Open a workspace folder first.');
	const bin = await ensureBinaryForCommand(context, ui.output);
	if (!bin) return;
	const res = await execNomos(bin, ['xray', '.', '--markdown'], root);
	if (res.code === 0 && res.stdout.trim()) {
		await vscode.env.clipboard.writeText(res.stdout.trim());
		vscode.window.showInformationMessage('Nomos X-Ray Markdown receipt copied to clipboard.');
	}
}

async function handleDocumentSaved(doc, context, diagnostics) {
	const { binaryPath, verifyOnSave } = getConfig();
	if (!verifyOnSave || doc.uri.scheme !== 'file') return;
	if (!SUPPORTED_EXTS.has(path.extname(doc.uri.fsPath).toLowerCase())) return;
	const root = getWorkspaceRoot();
	if (!root || !doc.uri.fsPath.startsWith(root)) return;
	const bin = resolveLocalBinary({ binaryPath, storageDir: context.globalStorageUri.fsPath, version: pkg.version });
	if (!bin) return;
	const rel = path.relative(root, doc.uri.fsPath);
	const res = await execNomos(bin, ['verify', '--fast-file', rel], root);
	diagnostics.delete(doc.uri);
	const items = parseFastFileRadarOutput(res.stdout, rel);
	if (items.length > 0) publishDiagnostics(diagnostics, root, items);
}

let activeUi = null;

/** Activates the lean Nomos VS Code extension. */
function activate(context) {
	const output = vscode.window.createOutputChannel('Nomos');
	const diagnostics = vscode.languages.createDiagnosticCollection('nomos');
	const statusBar = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 50);
	statusBar.command = 'nomos.xray';
	statusBar.text = formatStatusBarText(null);
	statusBar.tooltip = 'Click to run read-only Nomos Codebase X-Ray (0 files written)';
	statusBar.show();
	const ui = { output, diagnostics, statusBar };
	activeUi = ui;

	const mcpDisposable = registerMcpProvider(context);
	context.subscriptions.push(
		output,
		diagnostics,
		statusBar,
		vscode.workspace.onDidSaveTextDocument((doc) => void handleDocumentSaved(doc, context, diagnostics)),
		vscode.commands.registerCommand('nomos.xray', () => executeXrayCommand(context, ui)),
		vscode.commands.registerCommand('nomos.verify', () => executeVerifyCommand(context, ui)),
		vscode.commands.registerCommand('nomos.initLocal', () => executeInitCommand(context, ui, true)),
		vscode.commands.registerCommand('nomos.initTeam', () => executeInitCommand(context, ui, false)),
		vscode.commands.registerCommand('nomos.initVSCode', () => executeInitVSCodeHooksCommand(context, ui)),
		vscode.commands.registerCommand('nomos.copyXrayMarkdown', () => executeCopyMarkdownCommand(context, ui))
	);
	if (mcpDisposable) context.subscriptions.push(mcpDisposable);
}

/** Deactivates the Nomos extension and clears active diagnostic handles. */
function deactivate() {
	if (!activeUi) return;
	activeUi.diagnostics.clear();
	activeUi = null;
}

module.exports = { activate, deactivate };
