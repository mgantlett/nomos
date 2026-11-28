'use strict';
/**
 * binary.js - Zero-dependency binary resolver and SHA-256 verified downloader for the static Go `nomos` binary.
 *
 * Resolution order: explicit `nomos.binaryPath` -> system PATH -> cached globalStorage binary ->
 * explicit user-consented GitHub Release download verified against `checksums.txt`.
 */
const { execFileSync } = require('node:child_process');
const crypto = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');

const RELEASE_BASE_URL = 'https://github.com/mgantlett/nomos/releases/download';

/** Maps Node platform/arch onto the canonical GoReleaser archive triple. */
function resolveTargetPlatform(rawVersion, platform = process.platform, arch = process.arch) {
	const version = String(rawVersion || '').replace(/^v/, '').trim();
	const platMap = { linux: 'Linux', darwin: 'Darwin', win32: 'Windows' };
	const archMap = { x64: 'x86_64', arm64: 'arm64' };
	const goos = platMap[platform];
	const goarch = archMap[arch];
	if (!goos || !goarch) {
		throw new Error(`Unsupported OS/arch combination: ${platform}/${arch}`);
	}
	const ext = goos === 'Windows' ? 'zip' : 'tar.gz';
	const binName = goos === 'Windows' ? 'nomos.exe' : 'nomos';
	return { version, goos, goarch, ext, binName, archive: `nomos_${goos}_${goarch}.${ext}` };
}

/** Parses a GoReleaser checksums.txt manifest and returns the lowercase SHA-256 hex digest for archiveName. */
function parseExpectedSha256(checksumsText, archiveName) {
	for (const line of String(checksumsText || '').split(/\r?\n/)) {
		const trimmed = line.trim();
		if (!trimmed) continue;
		const parts = trimmed.split(/\s+/);
		if (parts.length >= 2 && parts[1].replace(/^\*/, '') === archiveName) {
			return parts[0].toLowerCase();
		}
	}
	return '';
}

/** Computes lowercase SHA-256 hex digest of a file on disk. */
function computeFileSha256(filePath) {
	const hash = crypto.createHash('sha256');
	hash.update(fs.readFileSync(filePath));
	return hash.digest('hex');
}

function isExecutableFile(candidate, platform = process.platform) {
	if (!candidate || !fs.existsSync(candidate)) return false;
	try {
		const stat = fs.statSync(candidate);
		if (!stat.isFile()) return false;
		if (platform !== 'win32') fs.accessSync(candidate, fs.constants.X_OK);
		return true;
	} catch (err) {
		void err;
		return false;
	}
}

/** Searches system PATH entries for an executable `nomos` binary. */
function findSystemBinary(envPath = process.env.PATH || '', platform = process.platform) {
	const binName = platform === 'win32' ? 'nomos.exe' : 'nomos';
	const sep = platform === 'win32' ? ';' : ':';
	for (const dir of String(envPath).split(sep).filter(Boolean)) {
		const candidate = path.join(dir, binName);
		if (isExecutableFile(candidate, platform)) return candidate;
	}
	return '';
}

/** Resolves an already-present local binary without performing any network I/O. */
function resolveLocalBinary(opts) {
	const cfg = opts || {};
	const platform = cfg.platform || process.platform;
	const explicit = String(cfg.binaryPath || '').trim();
	if (explicit) {
		return isExecutableFile(explicit, platform) ? explicit : '';
	}
	const onPath = findSystemBinary(cfg.envPath !== undefined ? cfg.envPath : process.env.PATH || '', platform);
	if (onPath) return onPath;
	if (cfg.storageDir && cfg.version) {
		const binName = platform === 'win32' ? 'nomos.exe' : 'nomos';
		const cached = path.join(cfg.storageDir, 'bin', `v${String(cfg.version).replace(/^v/, '')}`, binName);
		if (isExecutableFile(cached, platform)) return cached;
	}
	return '';
}

function extractArchive(archivePath, destDir, ext) {
	fs.mkdirSync(destDir, { recursive: true });
	if (ext === 'zip') {
		execFileSync('powershell', [
			'-NoProfile',
			'-NonInteractive',
			'-Command',
			`Expand-Archive -Path '${archivePath.replace(/'/g, "''")}' -DestinationPath '${destDir.replace(/'/g, "''")}' -Force`
		]);
		return;
	}
	execFileSync('tar', ['-xzf', archivePath, '-C', destDir]);
}

async function fetchBuffer(url, fetchFn) {
	const res = await fetchFn(url, {
		headers: { 'User-Agent': 'sophialabs-nomos-vscode' },
		signal: AbortSignal.timeout(30000)
	});
	if (!res.ok) {
		throw new Error(`HTTP ${res.status} fetching ${url}`);
	}
	return Buffer.from(await res.arrayBuffer());
}

/** Downloads and SHA-256 verifies the pinned static `nomos` release binary into `storageDir`. */
async function downloadVerifiedBinary(version, storageDir, opts = {}) {
	const fetchFn = opts.fetchFn || globalThis.fetch;
	const target = resolveTargetPlatform(version, opts.platform, opts.arch);
	const tag = `v${target.version}`;
	const base = opts.releaseBaseUrl || RELEASE_BASE_URL;
	const destDir = path.join(storageDir, 'bin', tag);
	const binPath = path.join(destDir, target.binName);
	if (isExecutableFile(binPath, opts.platform)) return binPath;

	const checksumsBuf = await fetchBuffer(`${base}/${tag}/checksums.txt`, fetchFn);
	const expected = parseExpectedSha256(checksumsBuf.toString('utf8'), target.archive);
	if (!expected) {
		throw new Error(`SHA-256 checksum missing for ${target.archive} in ${tag}/checksums.txt`);
	}

	const archiveBuf = await fetchBuffer(`${base}/${tag}/${target.archive}`, fetchFn);
	const tmpDir = path.join(storageDir, 'tmp', `${tag}-${Date.now()}`);
	fs.mkdirSync(tmpDir, { recursive: true });
	const archivePath = path.join(tmpDir, target.archive);
	try {
		fs.writeFileSync(archivePath, archiveBuf);
		const actual = computeFileSha256(archivePath);
		if (actual !== expected) {
			throw new Error(`SHA-256 mismatch for ${target.archive}: expected ${expected}, got ${actual}`);
		}
		extractArchive(archivePath, destDir, target.ext);
		if ((opts.platform || process.platform) !== 'win32') fs.chmodSync(binPath, 0o755);
		return binPath;
	} finally {
		fs.rmSync(tmpDir, { recursive: true, force: true });
	}
}

module.exports = {
	computeFileSha256,
	downloadVerifiedBinary,
	findSystemBinary,
	parseExpectedSha256,
	resolveLocalBinary,
	resolveTargetPlatform
};
