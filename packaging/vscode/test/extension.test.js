'use strict';
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const {
	computeFileSha256,
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

test('activate registers all 5 Nomos commands, status bar, and save watcher cleanly', () => {
	const Module = require('node:module');
	const registered = [];
	let cleared = false;
	const mockVscode = {
		StatusBarAlignment: { Left: 1 },
		DiagnosticSeverity: { Error: 0, Warning: 1, Information: 2 },
		window: {
			createOutputChannel: () => ({ appendLine: () => {}, clear: () => {}, show: () => {}, dispose: () => {} }),
			createStatusBarItem: () => ({ show: () => {}, dispose: () => {} })
		},
		languages: {
			createDiagnosticCollection: () => ({ clear: () => { cleared = true; }, delete: () => {}, set: () => {}, dispose: () => {} })
		},
		workspace: {
			workspaceFolders: [],
			getConfiguration: () => ({ get: () => '' }),
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
		const ext = require('../extension');
		const ctx = { subscriptions: [], globalStorageUri: { fsPath: os.tmpdir() } };
		ext.activate(ctx);
		assert.deepEqual(registered, [
			'nomos.xray',
			'nomos.verify',
			'nomos.initLocal',
			'nomos.initTeam',
			'nomos.copyXrayMarkdown'
		]);
		assert.equal(ctx.subscriptions.length, 9);
		ext.deactivate();
		assert.equal(cleared, true);
	} finally {
		Module._load = origLoad;
	}
});
