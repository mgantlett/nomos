'use strict';
const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const {
	computeFileSha256,
	downloadVerifiedBinary,
	parseExpectedSha256,
	resolveLocalBinary,
	resolveTargetPlatform
} = require('../lib/binary');
const {
	formatStatusBarText,
	parseFastFileRadarOutput,
	parseSarifDiagnostics,
	parseXrayReport,
	stripAnsi
} = require('../lib/parsers');

test('resolveTargetPlatform maps Linux, macOS, and Windows arch triples accurately', () => {
	const lin = resolveTargetPlatform('v4.34.1', 'linux', 'x64');
	assert.equal(lin.archive, 'nomos_Linux_x86_64.tar.gz');
	assert.equal(lin.binName, 'nomos');

	const mac = resolveTargetPlatform('4.34.1', 'darwin', 'arm64');
	assert.equal(mac.archive, 'nomos_Darwin_arm64.tar.gz');

	const win = resolveTargetPlatform('4.34.1', 'win32', 'x64');
	assert.equal(win.archive, 'nomos_Windows_x86_64.zip');
	assert.equal(win.binName, 'nomos.exe');
});

test('parseExpectedSha256 and computeFileSha256 verify release checksums deterministically', () => {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nomos-vscode-test-'));
	try {
		const sample = path.join(dir, 'sample.tar.gz');
		fs.writeFileSync(sample, 'nomos-release-payload');
		const digest = computeFileSha256(sample);
		const manifest = `${digest}  nomos_Linux_x86_64.tar.gz\n0000  nomos_Darwin_arm64.tar.gz\n`;
		assert.equal(parseExpectedSha256(manifest, 'nomos_Linux_x86_64.tar.gz'), digest);
		assert.equal(parseExpectedSha256(manifest, 'missing.tar.gz'), '');
	} finally {
		fs.rmSync(dir, { recursive: true, force: true });
	}
} );

test('resolveLocalBinary prioritizes explicit binaryPath then PATH then globalStorage cache', () => {
	const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nomos-vscode-bin-'));
	try {
		const explicitBin = path.join(dir, 'custom-nomos');
		const cachedDir = path.join(dir, 'storage', 'bin', 'v4.34.1');
		fs.mkdirSync(cachedDir, { recursive: true });
		const cachedBin = path.join(cachedDir, 'nomos');
		fs.writeFileSync(explicitBin, '#!/bin/sh\n', { mode: 0o755 });
		fs.writeFileSync(cachedBin, '#!/bin/sh\n', { mode: 0o755 });

		assert.equal(resolveLocalBinary({ binaryPath: explicitBin, platform: 'linux' }), explicitBin);
		assert.equal(resolveLocalBinary({ binaryPath: path.join(dir, 'nope'), platform: 'linux' }), '');
		assert.equal(
			resolveLocalBinary({ envPath: '', storageDir: path.join(dir, 'storage'), version: '4.34.1', platform: 'linux' }),
			cachedBin
		);
	} finally {
		fs.rmSync(dir, { recursive: true, force: true });
	}
});

test('parseFastFileRadarOutput strips ANSI and extracts file, line, severity, and message', () => {
	const sample = [
		'\x1b[31m🚨 [Nomos Radar Breach]\x1b[0m src/app.ts: src/app.ts:74: empty catch block silently swallows exception',
		'⚠️  [Nomos Radar >=80% Budget] src/app.ts: func handleClick complexity 14/15 (93% budget)'
	].join('\n');
	assert.equal(stripAnsi('\x1b[31mhi\x1b[0m'), 'hi');
	const items = parseFastFileRadarOutput(sample, 'src/app.ts');
	assert.equal(items.length, 2);
	assert.deepEqual(items[0], {
		file: 'src/app.ts',
		line: 74,
		col: 1,
		severity: 'error',
		ruleId: 'nomos-fast-radar',
		message: 'empty catch block silently swallows exception'
	});
	assert.equal(items[1].severity, 'warning');
	assert.equal(items[1].line, 1);
});

test('parseSarifDiagnostics and parseXrayReport normalize structured CLI payloads', () => {
	const sarif = JSON.stringify({
		runs: [{
			results: [{
				ruleId: 'G-16',
				level: 'error',
				message: { text: 'Hollow stub detected' },
				locations: [{ physicalLocation: { artifactLocation: { uri: 'pkg/foo.go' }, region: { startLine: 42, startColumn: 3 } } }]
			}]
		}]
	});
	const diags = parseSarifDiagnostics(sarif);
	assert.equal(diags.length, 1);
	assert.equal(diags[0].file, 'pkg/foo.go');
	assert.equal(diags[0].line, 42);
	assert.equal(diags[0].ruleId, 'G-16');

	const report = parseXrayReport(JSON.stringify({
		project_name: 'demo',
		grade: 'B',
		hangover_score: 18,
		files_scanned: 120,
		total_loc: 14500,
		duration_ms: 210,
		initialized: false,
		hotspots: [{ file: 'src/big.ts', loc: 480, cds: 52, reasons: ['monolith >400 LOC'] }]
	}));
	assert.equal(report.grade, 'B');
	assert.equal(report.hangoverScore, 18);
	assert.equal(report.hotspots.length, 1);
	assert.equal(formatStatusBarText(report), '$(shield) Nomos X-Ray: B (18)');
	assert.equal(formatStatusBarText(null), '$(shield) Nomos: Run X-Ray');
});

test('package.json declares workspace extensionKind, mcpServerDefinitionProviders, and zero dependencies or scripts', () => {
	const manifest = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
	assert.deepEqual(manifest.extensionKind, ['workspace', 'ui']);
	assert.equal(manifest.dependencies, undefined);
	assert.equal(manifest.devDependencies, undefined);
	assert.equal(manifest.scripts, undefined);
	assert.deepEqual(manifest.contributes.mcpServerDefinitionProviders, [
		{ id: 'nomos.mcp', label: 'Nomos (Read-Only Substrate MCP)' }
	]);
});

test('downloadVerifiedBinary verifies SHA-256 and rejects tampered archives offline', async () => {
	if (process.platform === 'win32') return;
	const workDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nomos-vscode-dl-'));
	const payloadDir = path.join(workDir, 'payload');
	fs.mkdirSync(payloadDir, { recursive: true });
	fs.writeFileSync(path.join(payloadDir, 'nomos'), '#!/bin/sh\necho nomos-offline\n', { mode: 0o755 });
	const target = resolveTargetPlatform('99.0.0', process.platform, process.arch);
	const tarPath = path.join(workDir, target.archive);
	execFileSync('tar', ['-czf', tarPath, '-C', payloadDir, 'nomos']);
	const goodSha = computeFileSha256(tarPath);
	const badSha = '0'.repeat(64);

	const srv = http.createServer((req, res) => {
		if (req.url === '/bad/v99.0.0/checksums.txt') {
			res.end(`${badSha}  ${target.archive}\n`);
			return;
		}
		if (req.url === '/good/v99.0.0/checksums.txt') {
			res.end(`${goodSha}  ${target.archive}\n`);
			return;
		}
		if (req.url && req.url.endsWith(`/${target.archive}`)) {
			res.end(fs.readFileSync(tarPath));
			return;
		}
		res.statusCode = 404;
		res.end();
	});
	await new Promise((resolve) => srv.listen(0, '127.0.0.1', resolve));
	const base = `http://127.0.0.1:${srv.address().port}`;
	try {
		await assert.rejects(
			() => downloadVerifiedBinary('99.0.0', path.join(workDir, 'store-bad'), { releaseBaseUrl: `${base}/bad` }),
			/SHA-256 mismatch/
		);
		const bin = await downloadVerifiedBinary('99.0.0', path.join(workDir, 'store-good'), { releaseBaseUrl: `${base}/good` });
		assert.equal(fs.existsSync(bin), true);
	} finally {
		srv.close();
		fs.rmSync(workDir, { recursive: true, force: true });
	}
});

test('activate registers all 6 Nomos commands, MCP provider, status bar, and save watcher cleanly', () => {
	const Module = require('node:module');
	const registered = [];
	let mcpProvider = null;
	let cleared = false;
	const fakeBinDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nomos-vscode-act-'));
	const fakeBin = path.join(fakeBinDir, 'nomos');
	fs.writeFileSync(fakeBin, '#!/bin/sh\n', { mode: 0o755 });
	class MockMcpStdioServerDefinition {
		constructor(label, command, args, version) {
			this.label = label;
			this.command = command;
			this.args = args;
			this.version = version;
		}
	}
	const mockVscode = {
		StatusBarAlignment: { Left: 1 },
		DiagnosticSeverity: { Error: 0, Warning: 1, Information: 2 },
		Uri: { file: (fsPath) => ({ fsPath }) },
		McpStdioServerDefinition: MockMcpStdioServerDefinition,
		lm: {
			registerMcpServerDefinitionProvider: (id, provider) => {
				mcpProvider = { id, provider };
				return { dispose: () => {} };
			}
		},
		window: {
			createOutputChannel: () => ({ appendLine: () => {}, clear: () => {}, show: () => {}, dispose: () => {} }),
			createStatusBarItem: () => ({ show: () => {}, dispose: () => {} })
		},
		languages: {
			createDiagnosticCollection: () => ({ clear: () => { cleared = true; }, delete: () => {}, set: () => {}, dispose: () => {} })
		},
		workspace: {
			workspaceFolders: [{ uri: { fsPath: fakeBinDir } }],
			getConfiguration: () => ({ get: (key) => (key === 'binaryPath' ? fakeBin : '') }),
			onDidSaveTextDocument: () => ({ dispose: () => {} })
		},
		commands: {
			registerCommand: (id, fn) => {
				registered.push(id);
				return { id, fn, dispose: () => {} };
			}
		}
	};
	const origLoad = Module._load;
	Module._load = function patchedLoad(request, parent, isMain) {
		if (request === 'vscode') return mockVscode;
		return origLoad.call(this, request, parent, isMain);
	};
	try {
		delete require.cache[require.resolve('../extension')];
		const ext = require('../extension');
		const ctx = { subscriptions: [], globalStorageUri: { fsPath: os.tmpdir() } };
		ext.activate(ctx);
		assert.deepEqual(registered, [
			'nomos.xray',
			'nomos.verify',
			'nomos.initLocal',
			'nomos.initTeam',
			'nomos.initVSCode',
			'nomos.copyXrayMarkdown'
		]);
		assert.equal(ctx.subscriptions.length, 11);
		assert.equal(mcpProvider.id, 'nomos.mcp');
		const defs = mcpProvider.provider.provideMcpServerDefinitions();
		assert.equal(defs.length, 1);
		assert.equal(defs[0].command, fakeBin);
		assert.deepEqual(defs[0].args, ['mcp']);
		assert.equal(defs[0].cwd.fsPath, fakeBinDir);
		ext.deactivate();
		assert.equal(cleared, true);
	} finally {
		Module._load = origLoad;
		fs.rmSync(fakeBinDir, { recursive: true, force: true });
	}
});
