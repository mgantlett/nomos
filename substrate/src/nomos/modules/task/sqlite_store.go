// Package task defines domain models, schema constraints, and tracking mechanisms
// enforced by compiled Go SSoT contracts.
package task

import (
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
)

// SQLiteTaskStore implements TaskStore backed by pure-Go SQLite.
type SQLiteTaskStore struct {
	db *sql.DB
	mu sync.Mutex
}

// NewSQLiteTaskStore instantiates a new SQLite-backed TaskStore.
func NewSQLiteTaskStore(db *sql.DB) *SQLiteTaskStore {
	return &SQLiteTaskStore{db: db}
}

// NewDefaultSQLiteTaskStore opens the global Nomos database dynamically.
func NewDefaultSQLiteTaskStore() (*SQLiteTaskStore, error) {
	db, err := storage.OpenDB("")
	if err != nil {
		return nil, err
	}
	return NewSQLiteTaskStore(db), nil
}

// CreateTask registers a new task in SQLite, enforcing structural schema invariants.
func (s *SQLiteTaskStore) CreateTask(t TaskRecord) error {
	if err := ValidateTaskRecord(&t); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	depJSON, err := json.Marshal(t.Dependencies)
	if err != nil {
		depJSON = []byte("[]")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	if t.UpdatedAt == "" {
		t.UpdatedAt = now
	}
	if t.Status == "" {
		t.Status = StatusReady
	}
	if t.Tier == "" {
		t.Tier = Tier1
	}

	query := `INSERT INTO tasks (
		key, plane, project, schema_type, title, description, status, tier, priority,
		created_at, updated_at, completed_at, dependencies, twin_key, arc_id,
		intent_delta, burden, context_burden, arch_layer
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query,
		t.Key, t.Plane, t.Project, string(t.SchemaType), t.Title, t.Description,
		string(t.Status), string(t.Tier), t.Priority, t.CreatedAt, t.UpdatedAt,
		t.CompletedAt, string(depJSON), t.TwinKey, t.ArcID,
		t.IntentDelta, t.Burden, t.ContextBurden, t.ArchLayer,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return types.NewNomosError(types.ErrCodeInvariant, "task %s already exists", t.Key)
		}
		return types.NewNomosError(types.ErrCodeSubstrate, "failed to insert task %s: %v", t.Key, err)
	}
	return nil
}

// GetTask retrieves a task record by canonical key.
func (s *SQLiteTaskStore) GetTask(key string) (*TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, key, plane, project, schema_type, title, description, status, tier, priority,
		created_at, updated_at, completed_at, dependencies, twin_key, arc_id,
		intent_delta, burden, context_burden, arch_layer FROM tasks WHERE key = ?`

	row := s.db.QueryRow(query, key)
	rec, err := scanTaskRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, types.NewNomosError(types.ErrCodeValidationFailed, "task %s not found", key)
		}
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed to get task %s: %v", key, err)
	}
	return rec, nil
}

// UpdateTask updates an existing task record.
func (s *SQLiteTaskStore) UpdateTask(t TaskRecord) error {
	if err := ValidateTaskRecord(&t); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	depJSON, err := json.Marshal(t.Dependencies)
	if err != nil {
		depJSON = []byte("[]")
	}
	t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	query := `UPDATE tasks SET
		project = ?, schema_type = ?, title = ?, description = ?, status = ?, tier = ?,
		priority = ?, updated_at = ?, completed_at = ?, dependencies = ?,
		twin_key = ?, arc_id = ?, intent_delta = ?, burden = ?,
		context_burden = ?, arch_layer = ?
		WHERE key = ?`

	res, err := s.db.Exec(query,
		t.Project, string(t.SchemaType), t.Title, t.Description, string(t.Status), string(t.Tier),
		t.Priority, t.UpdatedAt, t.CompletedAt, string(depJSON),
		t.TwinKey, t.ArcID, t.IntentDelta, t.Burden,
		t.ContextBurden, t.ArchLayer, t.Key,
	)
	if err != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed to update task %s: %v", t.Key, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed to verify rows affected: %v", err)
	}
	if rows == 0 {
		return types.NewNomosError(types.ErrCodeValidationFailed, "task %s not found", t.Key)
	}
	return nil
}

// ListTasks returns all registered task records.
func (s *SQLiteTaskStore) ListTasks() []TaskRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, key, plane, project, schema_type, title, description, status, tier, priority,
		created_at, updated_at, completed_at, dependencies, twin_key, arc_id,
		intent_delta, burden, context_burden, arch_layer FROM tasks ORDER BY id ASC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			// ignore or log
		}
	}()

	var list []TaskRecord
	for rows.Next() {
		rec, err := scanTaskRow(rows)
		if err == nil && rec != nil {
			list = append(list, *rec)
		}
	}
	return list
}

// AddDependency declares that targetKey depends on blockedByKey.
func (s *SQLiteTaskStore) AddDependency(targetKey, blockedByKey string) error {
	target, err := s.GetTask(targetKey)
	if err != nil {
		return err
	}
	if _, err := s.GetTask(blockedByKey); err != nil {
		return err
	}

	for _, dep := range target.Dependencies {
		if dep == blockedByKey {
			return nil
		}
	}
	target.Dependencies = append(target.Dependencies, blockedByKey)
	return s.UpdateTask(*target)
}

// GetWavefront returns unblocked candidate tasks ready for concurrent execution.
func (s *SQLiteTaskStore) GetWavefront() ([]WavefrontCandidate, error) {
	allTasks := s.ListTasks()
	allMap := make(map[string]TaskRecord, len(allTasks))
	for _, t := range allTasks {
		allMap[t.Key] = t
	}

	var wavefront []WavefrontCandidate
	for _, t := range allTasks {
		if t.Status == StatusDone || t.Status == StatusAborted {
			continue
		}
		if isTaskUnblocked(t, allMap) {
			wavefront = append(wavefront, WavefrontCandidate{
				Key:         t.Key,
				Title:       t.Title,
				Plane:       t.Plane,
				Project:     t.Project,
				SchemaType:  t.SchemaType,
				Status:      t.Status,
				Tier:        t.Tier,
				TwinKey:     t.TwinKey,
				IsUnblocked: true,
			})
		}
	}
	return wavefront, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTaskRow(scanner rowScanner) (*TaskRecord, error) {
	var (
		rec         TaskRecord
		nullID      sql.NullInt64
		project     sql.NullString
		schemaType  sql.NullString
		desc        sql.NullString
		status      sql.NullString
		tier        sql.NullString
		priority    sql.NullInt64
		completedAt sql.NullString
		depJSON     sql.NullString
		twinKey     sql.NullString
		arcID       sql.NullString
		intentDelta sql.NullString
		burden      sql.NullInt64
		ctxBurden   sql.NullInt64
		archLayer   sql.NullInt64
	)
	err := scanner.Scan(
		&nullID, &rec.Key, &rec.Plane, &project, &schemaType, &rec.Title, &desc,
		&status, &tier, &priority, &rec.CreatedAt, &rec.UpdatedAt,
		&completedAt, &depJSON, &twinKey, &arcID,
		&intentDelta, &burden, &ctxBurden, &archLayer,
	)
	if err != nil {
		return nil, err
	}
	rec.Project = project.String

	mapScannedFields(&rec, nullID, schemaType, desc, status, tier, twinKey, priority)
	mapScannedMetadata(&rec, completedAt, depJSON, arcID, intentDelta, burden, ctxBurden, archLayer)
	return &rec, nil
}

func mapScannedFields(rec *TaskRecord, nullID sql.NullInt64, schemaType, desc, status, tier, twinKey sql.NullString, priority sql.NullInt64) {
	if nullID.Valid {
		rec.ID = int(nullID.Int64)
	}
	rec.Description = desc.String
	rec.TwinKey = twinKey.String
	rec.SchemaType = SchemaType(schemaType.String)
	if rec.SchemaType == "" {
		rec.SchemaType = SchemaType(rec.Plane)
	}
	rec.Status = Status(status.String)
	rec.Tier = Tier(tier.String)
	if rec.Tier == "" {
		rec.Tier = Tier1
	}
	rec.Priority = 50
	if priority.Valid {
		rec.Priority = int(priority.Int64)
	}
}

func mapScannedMetadata(rec *TaskRecord, completedAt, depJSON, arcID, intentDelta sql.NullString, burden, ctxBurden, archLayer sql.NullInt64) {
	if completedAt.Valid {
		s := completedAt.String
		rec.CompletedAt = &s
	}
	rec.ArcID = arcID.String
	rec.IntentDelta = intentDelta.String
	rec.Burden = int(burden.Int64)
	rec.ContextBurden = int(ctxBurden.Int64)
	rec.ArchLayer = int(archLayer.Int64)
	rec.Dependencies = []string{}
	if depJSON.Valid && depJSON.String != "" {
		if err := json.Unmarshal([]byte(depJSON.String), &rec.Dependencies); err != nil {
			rec.Dependencies = []string{}
		}
	}
}

// Close terminates the underlying SQLite database connection.
func (s *SQLiteTaskStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// BindTwin establishes bilateral twin links between two tasks.
func (s *SQLiteTaskStore) BindTwin(keyA, keyB string) error {
	tA, errA := s.GetTask(keyA)
	if errA != nil {
		return errA
	}
	tB, errB := s.GetTask(keyB)
	if errB != nil {
		return errB
	}
	tA.TwinKey = keyB
	tB.TwinKey = keyA
	if err := s.UpdateTask(*tA); err != nil {
		return err
	}
	return s.UpdateTask(*tB)
}


