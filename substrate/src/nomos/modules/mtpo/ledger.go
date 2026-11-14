// Package mtpo persists worktree MTPO trajectories and AST read/write sets in SQLite.
package mtpo

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
)

// AllocateSequence assigns the next monotonic MTPO sequence for an active worktree.
// @intent software_physics_engine_spec.md #MTPOLedgerEntry
// @intent nomos_hermetic_worktree_spec.md
func AllocateSequence(dbPath, repoRoot, taskKey, worktreePath, baseCommit string) (*MTPOLedgerEntry, error) {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()
	canonRepo := normalizePathSlash(repoRoot)
	canonWT := normalizePathSlash(worktreePath)
	upperKey := strings.ToUpper(strings.TrimSpace(taskKey))
	now := time.Now().UTC().Format(time.RFC3339)

	if _, uErr := db.Exec(
		`UPDATE worktree_mtpo SET status = ?, updated_at = ? WHERE repo_root = ? AND task_key = ? AND status = ?`,
		StatusAborted, now, canonRepo, upperKey, StatusActive,
	); uErr != nil {
		return nil, uErr
	}
	snapSeq := readLatestCommitSeq(db, canonRepo)
	res, iErr := db.Exec(
		`INSERT INTO worktree_mtpo (repo_root, task_key, worktree_path, base_commit, base_commit_seq, status, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		canonRepo, upperKey, canonWT, strings.TrimSpace(baseCommit), snapSeq, StatusActive, now,
	)
	if iErr != nil {
		return nil, iErr
	}
	seq, lErr := res.LastInsertId()
	if lErr != nil {
		return nil, lErr
	}
	return &MTPOLedgerEntry{
		Seq:               seq,
		RepoRoot:          canonRepo,
		TaskKey:           upperKey,
		WorktreePath:      canonWT,
		BaseCommit:        strings.TrimSpace(baseCommit),
		Status:            StatusActive,
		WriteFiles:        []string{},
		WriteSymbols:      []string{},
		ReadSymbols:       []string{},
		ReferencedImports: []string{},
		UpdatedAt:         now,
	}, nil
}

// RecordFileMutation updates AST symbol and import sets and returns advisory MTPO conflicts.
// @intent software_physics_engine_spec.md #MTPOConflict
// @intent nomos_hermetic_worktree_spec.md
func RecordFileMutation(dbPath, repoRoot, taskKey, worktreePath, editedFile string) ([]MTPOConflict, error) {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()
	canonRepo := normalizePathSlash(repoRoot)
	canonWT := normalizePathSlash(worktreePath)
	upperKey := strings.ToUpper(strings.TrimSpace(taskKey))
	current, gErr := ensureActiveEntry(db, canonRepo, upperKey, canonWT)
	if gErr != nil {
		return nil, gErr
	}
	relFile := computeWorktreeRelPath(worktreePath, editedFile)
	wSyms, rSyms, imps := ExtractFileASTSets(editedFile, relFile)
	if !strings.HasSuffix(relFile, "_test.go") {
		current.WriteFiles = mergeSortedStringSets(current.WriteFiles, []string{relFile})
	}
	current.WriteSymbols = mergeSortedStringSets(current.WriteSymbols, wSyms)
	current.ReadSymbols = mergeSortedStringSets(current.ReadSymbols, rSyms)
	current.ReferencedImports = mergeSortedStringSets(current.ReferencedImports, imps)
	current.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if uErr := saveEntrySets(db, current); uErr != nil {
		return nil, uErr
	}
	peers, lErr := queryEntriesByStatus(db, canonRepo, StatusActive)
	if lErr != nil {
		return nil, lErr
	}
	return EvaluateActiveHazards(current, peers), nil
}

// UpdateBaseCommit refreshes the verified trunk base commit after rebase and DoD re-verification.
func UpdateBaseCommit(dbPath, repoRoot, taskKey, newBaseCommit string) error {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()
	canonRepo := normalizePathSlash(repoRoot)
	snapSeq := readLatestCommitSeq(db, canonRepo)
	_, eErr := db.Exec(
		`UPDATE worktree_mtpo SET base_commit = ?, base_commit_seq = ?, updated_at = ? WHERE repo_root = ? AND task_key = ? AND status = ?`,
		strings.TrimSpace(newBaseCommit), snapSeq, time.Now().UTC().Format(time.RFC3339),
		canonRepo, strings.ToUpper(strings.TrimSpace(taskKey)), StatusActive,
	)
	return eErr
}

// CompleteSequence marks an active worktree MTPO sequence as COMMITTED or ABORTED.
func CompleteSequence(dbPath, repoRoot, taskKey, finalStatus, mergedCommit string) error {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()
	canonRepo := normalizePathSlash(repoRoot)
	nextCommitSeq := int64(0)
	if finalStatus == StatusCommitted {
		nextCommitSeq = readLatestCommitSeq(db, canonRepo) + 1
	}
	_, eErr := db.Exec(
		`UPDATE worktree_mtpo SET status = ?, merged_commit = ?, commit_seq = ?, updated_at = ? WHERE repo_root = ? AND task_key = ? AND status = ?`,
		finalStatus, strings.TrimSpace(mergedCommit), nextCommitSeq, time.Now().UTC().Format(time.RFC3339),
		canonRepo, strings.ToUpper(strings.TrimSpace(taskKey)), StatusActive,
	)
	return eErr
}

func readLatestCommitSeq(db *sql.DB, canonRepo string) int64 {
	var maxSeq int64
	if err := db.QueryRow(
		`SELECT COALESCE(MAX(commit_seq), 0) FROM worktree_mtpo WHERE repo_root = ? AND status = ?`,
		canonRepo, StatusCommitted,
	).Scan(&maxSeq); err != nil {
		return 0
	}
	return maxSeq
}

// ListActiveEntries returns active MTPO trajectories ordered ascending by monotonic sequence.
func ListActiveEntries(dbPath, repoRoot string) ([]MTPOLedgerEntry, error) {
	db, err := storage.OpenDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := db.Close(); cErr != nil {
			return
		}
	}()
	return queryEntriesByStatus(db, normalizePathSlash(repoRoot), StatusActive)
}

func ensureActiveEntry(db *sql.DB, canonRepo, upperKey, canonWT string) (*MTPOLedgerEntry, error) {
	entry, err := queryLatestTaskEntry(db, canonRepo, upperKey, StatusActive)
	if err == nil && entry != nil {
		return entry, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, iErr := db.Exec(
		`INSERT INTO worktree_mtpo (repo_root, task_key, worktree_path, base_commit, status, updated_at) VALUES (?, ?, ?, '', ?, ?)`,
		canonRepo, upperKey, canonWT, StatusActive, now,
	)
	if iErr != nil {
		return nil, iErr
	}
	seq, lErr := res.LastInsertId()
	if lErr != nil {
		return nil, lErr
	}
	return &MTPOLedgerEntry{
		Seq: seq, RepoRoot: canonRepo, TaskKey: upperKey, WorktreePath: canonWT,
		Status: StatusActive, UpdatedAt: now,
	}, nil
}

func saveEntrySets(db *sql.DB, e *MTPOLedgerEntry) error {
	_, err := db.Exec(
		`UPDATE worktree_mtpo SET write_files = ?, write_symbols = ?, read_symbols = ?, referenced_imports = ?, updated_at = ? WHERE seq = ?`,
		encodeJSONSlice(e.WriteFiles), encodeJSONSlice(e.WriteSymbols),
		encodeJSONSlice(e.ReadSymbols), encodeJSONSlice(e.ReferencedImports),
		e.UpdatedAt, e.Seq,
	)
	return err
}

func encodeJSONSlice(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	raw, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func queryLatestTaskEntry(db *sql.DB, canonRepo, upperKey, status string) (*MTPOLedgerEntry, error) {
	row := db.QueryRow(
		`SELECT seq, repo_root, task_key, worktree_path, base_commit, status, write_files, write_symbols, read_symbols, referenced_imports, updated_at
		 FROM worktree_mtpo WHERE repo_root = ? AND task_key = ? AND status = ? ORDER BY seq DESC LIMIT 1`,
		canonRepo, upperKey, status,
	)
	return scanSingleEntry(row)
}

func queryEntriesByStatus(db *sql.DB, canonRepo, status string) ([]MTPOLedgerEntry, error) {
	rows, err := db.Query(
		`SELECT seq, repo_root, task_key, worktree_path, base_commit, status, write_files, write_symbols, read_symbols, referenced_imports, updated_at
		 FROM worktree_mtpo WHERE repo_root = ? AND status = ? ORDER BY seq ASC`,
		canonRepo, status,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cErr := rows.Close(); cErr != nil {
			return
		}
	}()
	var out []MTPOLedgerEntry
	for rows.Next() {
		e, sErr := scanEntryRows(rows)
		if sErr != nil {
			return nil, sErr
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSingleEntry(r rowScanner) (*MTPOLedgerEntry, error) {
	return scanEntryRows(r)
}

func scanEntryRows(r rowScanner) (*MTPOLedgerEntry, error) {
	var e MTPOLedgerEntry
	var wf, ws, rs, ri string
	if err := r.Scan(&e.Seq, &e.RepoRoot, &e.TaskKey, &e.WorktreePath, &e.BaseCommit, &e.Status, &wf, &ws, &rs, &ri, &e.UpdatedAt); err != nil {
		return nil, err
	}
	e.WriteFiles = decodeJSONSlice(wf)
	e.WriteSymbols = decodeJSONSlice(ws)
	e.ReadSymbols = decodeJSONSlice(rs)
	e.ReferencedImports = decodeJSONSlice(ri)
	return &e, nil
}

func decodeJSONSlice(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return []string{}
	}
	return out
}

func normalizePathSlash(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return ""
	}
	if abs, err := filepath.Abs(trimmed); err == nil {
		return filepath.ToSlash(filepath.Clean(abs))
	}
	return filepath.ToSlash(filepath.Clean(trimmed))
}

func computeWorktreeRelPath(worktreePath, editedFile string) string {
	absWT, wErr := filepath.Abs(worktreePath)
	absFile, fErr := filepath.Abs(editedFile)
	if wErr == nil && fErr == nil {
		if rel, rErr := filepath.Rel(absWT, absFile); rErr == nil && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(filepath.Base(editedFile))
}
