'use strict';
/**
 * parsers.js - Pure zero-dependency parsers for `nomos xray --json`, `nomos verify --fast-file`,
 * and `nomos verify --sarif` output consumed by VS Code diagnostics and the status bar.
 */

const ANSI_RE = /\x1b\[[0-9;]*[a-zA-Z]/g;
const RADAR_LINE_RE = /\[Nomos Radar (Breach|>=80% Budget)\]\s+([^:\s]+):\s+(.+)$/;
const EMBEDDED_LOC_RE = /^([^:\s]+):(\d+)(?:-\d+)?(?::\d+)?\s*:?\s*(.+)$/;

/** Strips ANSI terminal escape codes cleanly. */
function stripAnsi(raw) {
	return String(raw || '').replace(ANSI_RE, '');
}

/** Parses `nomos verify --fast-file` stdout lines into structured Problems diagnostics. */
function parseFastFileRadarOutput(stdout, fallbackFile = '') {
	const items = [];
	for (const rawLine of stripAnsi(stdout).split(/\r?\n/)) {
		const trimmed = rawLine.trim();
		const match = trimmed.match(RADAR_LINE_RE);
		if (!match) continue;
		const severity = match[1] === 'Breach' ? 'error' : 'warning';
		let file = match[2] || fallbackFile;
		let line = 1;
		let message = match[3].trim();
		const locMatch = message.match(EMBEDDED_LOC_RE);
		if (locMatch) {
			file = locMatch[1] || file;
			line = Math.max(1, Number.parseInt(locMatch[2], 10) || 1);
			message = locMatch[3].trim();
		}
		items.push({ file, line, col: 1, severity, ruleId: 'nomos-fast-radar', message });
	}
	return items;
}

function normalizeSarifResult(res) {
	const loc = res?.locations?.[0]?.physicalLocation;
	const uri = String(loc?.artifactLocation?.uri || '').trim();
	if (!uri) return null;
	const line = Math.max(1, Number(loc?.region?.startLine) || 1);
	const col = Math.max(1, Number(loc?.region?.startColumn) || 1);
	const level = String(res?.level || 'warning').toLowerCase();
	const severity = level === 'error' ? 'error' : level === 'note' ? 'info' : 'warning';
	return {
		file: uri,
		line,
		col,
		severity,
		ruleId: String(res?.ruleId || 'nomos'),
		message: String(res?.message?.text || 'Nomos quality gate finding')
	};
}

/** Parses OASIS SARIF 2.1.0 JSON emitted by `nomos verify --sarif` or `nomos xray --sarif`. */
function parseSarifDiagnostics(rawJson) {
	const trimmed = typeof rawJson === 'string' ? rawJson.trim() : '';
	if (typeof rawJson === 'string' && !trimmed.startsWith('{')) return [];
	let doc = rawJson;
	if (typeof rawJson === 'string') {
		try {
			doc = JSON.parse(trimmed);
		} catch (err) {
			console.warn('[nomos] invalid SARIF payload:', err instanceof Error ? err.message : String(err));
			return [];
		}
	}
	const items = [];
	for (const run of Array.isArray(doc?.runs) ? doc.runs : []) {
		for (const res of Array.isArray(run?.results) ? run.results : []) {
			const entry = normalizeSarifResult(res);
			if (entry) items.push(entry);
		}
	}
	return items;
}

function normalizeXrayHotspot(h) {
	return {
		file: String(h?.file || ''),
		loc: Number(h?.loc || 0),
		cds: Number(h?.cds || 0),
		reasons: Array.isArray(h?.reasons) ? h.reasons.map(String) : []
	};
}

/** Parses `nomos xray . --json` output into a normalized summary + hotspot list. */
function parseXrayReport(rawJson) {
	const doc = typeof rawJson === 'string' ? JSON.parse(rawJson) : rawJson;
	const rawHotspots = Array.isArray(doc?.hotspots) ? doc.hotspots : [];
	return {
		projectName: String(doc?.project_name || ''),
		grade: String(doc?.grade || 'A'),
		hangoverScore: Number(doc?.hangover_score || 0),
		filesScanned: Number(doc?.files_scanned || 0),
		totalLoc: Number(doc?.total_loc || 0),
		durationMs: Number(doc?.duration_ms || 0),
		initialized: Boolean(doc?.initialized),
		hotspots: rawHotspots.map(normalizeXrayHotspot)
	};
}

/** Formats the compact VS Code status bar text for a completed X-Ray report. */
function formatStatusBarText(report) {
	if (!report) return '$(shield) Nomos: Run X-Ray';
	return `$(shield) Nomos X-Ray: ${report.grade} (${report.hangoverScore})`;
}

module.exports = {
	formatStatusBarText,
	parseFastFileRadarOutput,
	parseSarifDiagnostics,
	parseXrayReport,
	stripAnsi
};
