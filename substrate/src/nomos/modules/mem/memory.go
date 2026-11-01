// Package mem implements the Nomos Mneme episodic and vector memory system.
package mem

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	_ "modernc.org/sqlite"
)

// MemoryKind categorizes cognitive memory entries.
type MemoryKind string

const (
	KindReflection   MemoryKind = "reflection"
	KindAxiom        MemoryKind = "axiom"
	KindDecision     MemoryKind = "decision"
	KindFact         MemoryKind = "fact"
	KindPreference   MemoryKind = "preference"
	KindArchitecture MemoryKind = "architecture"
	KindTask         MemoryKind = "task"
	KindCommit       MemoryKind = "commit"
	KindRelease      MemoryKind = "release"
	KindReview       MemoryKind = "review"
	KindLegacySpec   MemoryKind = "legacy_spec"
	KindPitfall      MemoryKind = "pitfall"
	KindPattern      MemoryKind = "pattern"
)

// ValidateAdvisoryWriteKind enforces statutory write provenance for manual CLI saves (@intent: #MnemeMemory).
func ValidateAdvisoryWriteKind(kind MemoryKind) error {
	normalized := MemoryKind(strings.ToLower(strings.TrimSpace(string(kind))))
	switch normalized {
	case "", KindReflection, KindAxiom, KindDecision, KindFact, KindPreference, KindArchitecture:
		return nil
	case KindPitfall, KindPattern, KindRelease, KindTask, KindCommit, KindReview, KindLegacySpec:
		return types.NewNomosError(
			types.ErrCodeValidationFailed,
			"memory kind %q is statutory and minted strictly by 2PC 'nomos task sync'",
			normalized,
		).WithRemedy("use an advisory kind (--kind reflection, --kind axiom, or --kind decision) for manual 'nomos mem save'")
	default:
		return types.NewNomosError(types.ErrCodeValidationFailed, "unsupported memory kind %q", kind).
			WithRemedy("valid advisory kinds: reflection, axiom, decision, fact, preference, architecture")
	}
}

// MemoryStatus defines the lifecycle state of an episodic memory.
type MemoryStatus string

const (
	StatusActive     MemoryStatus = "active"
	StatusSuperseded MemoryStatus = "superseded"
	StatusTombstoned MemoryStatus = "tombstoned"
)

// Memory represents a single knowledge item indexed in Nomos Mneme (#MnemeMemory).
type Memory struct {
	ID          int          `json:"id"`
	RepoID      string       `json:"repo_id,omitempty"`
	Kind        MemoryKind   `json:"kind"`
	Topic       string       `json:"topic"`
	Content     string       `json:"content"`
	TaskID      string       `json:"task_id"`
	Embedding   []float32    `json:"embedding,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	Status       MemoryStatus `json:"status"`
	SupersededBy int          `json:"superseded_by,omitempty"`
	AccessCount  int          `json:"access_count"`
}

// MemoryFilter encapsulates criteria for querying stored memories.
type MemoryFilter struct {
	TaskID           string
	Kind             MemoryKind
	Topic            string
	Status           MemoryStatus
	RepoID           string
	IncludeCrossRepo bool
	Limit            int
}

// Store provides pure-Go SQLite persistence for Nomos Mneme memories.
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

const memSchemaDDL = `
CREATE TABLE IF NOT EXISTS memories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	repo_id TEXT DEFAULT '',
	kind TEXT NOT NULL,
	topic TEXT NOT NULL,
	content TEXT NOT NULL,
	task_id TEXT NOT NULL,
	embedding BLOB,
	created_at DATETIME NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	superseded_by INTEGER DEFAULT 0,
	access_count INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_memories_topic ON memories(topic);
CREATE INDEX IF NOT EXISTS idx_memories_status ON memories(status);
CREATE INDEX IF NOT EXISTS idx_memories_repo ON memories(repo_id);
CREATE INDEX IF NOT EXISTS idx_memories_kind ON memories(kind);
CREATE INDEX IF NOT EXISTS idx_memories_repo_kind_status ON memories(repo_id, kind, status);
`

// NewStore initializes a SQLite Nomos Mneme memory store.
func NewStore(dbPath string) (*Store, error) {
	target := dbPath
	if target == "" {
		target = config.ResolveMnemeDBPath()
	}

	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed creating brain db dir %s: %v", dir, err)
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", target)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed opening mneme db: %v", err)
	}

	if _, err := db.Exec(memSchemaDDL); err != nil {
		if cErr := db.Close(); cErr != nil {
			return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed initializing mem schema: %v (close failed: %v)", err, cErr)
		}
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed initializing mem schema: %v", err)
	}

	return &Store{db: db}, nil
}

// Close terminates the SQLite connection.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Save inserts a memory record and its vector embedding into Nomos Mneme, deduplicating active clones idempotently.
func (s *Store) Save(m *Memory) error {
	if err := prepareMemoryForSave(m); err != nil {
		return err
	}

	embBytes := EncodeVector(m.Embedding)
	s.mu.Lock()
	defer s.mu.Unlock()

	if m.Status == StatusActive {
		if existingID := s.findActiveDuplicateIDLocked(m.RepoID, m.Kind, m.Content); existingID > 0 {
			s.reinforceExistingLocked(existingID)
			m.ID = existingID
			return nil
		}
	}

	query := `INSERT INTO memories (repo_id, kind, topic, content, task_id, embedding, created_at, status, superseded_by, access_count)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.Exec(query, strings.TrimSpace(m.RepoID), string(m.Kind), strings.TrimSpace(m.Topic),
		strings.TrimSpace(m.Content), strings.ToUpper(strings.TrimSpace(m.TaskID)),
		embBytes, m.CreatedAt.Format(time.RFC3339), string(m.Status), m.SupersededBy, m.AccessCount)
	if err != nil {
		return types.NewNomosError(types.ErrCodeSubstrate, "failed inserting memory: %v", err)
	}

	if id, idErr := res.LastInsertId(); idErr == nil {
		m.ID = int(id)
	}
	return nil
}

func prepareMemoryForSave(m *Memory) error {
	if m == nil {
		return types.NewNomosError(types.ErrCodeValidationFailed, "cannot save nil memory")
	}
	if strings.TrimSpace(m.Content) == "" {
		return types.NewNomosError(types.ErrCodeValidationFailed, "memory content cannot be empty")
	}
	if m.Kind == "" {
		m.Kind = KindReflection
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	if m.Status == "" {
		m.Status = StatusActive
	}
	if len(m.Embedding) == 0 {
		emb, err := GenerateEmbedding(m.Content)
		if err != nil {
			return types.NewNomosError(types.ErrCodeSubstrate, "failed generating embedding: %v", err)
		}
		m.Embedding = emb
	}
	return nil
}

// Get retrieves a memory record by ID.
func (s *Store) Get(id int) (*Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, repo_id, kind, topic, content, task_id, embedding, created_at, status, superseded_by, access_count
	          FROM memories WHERE id = ?`
	row := s.db.QueryRow(query, id)
	return scanMemoryRow(row)
}

// ListAll returns all stored memories.
func (s *Store) ListAll() ([]*Memory, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, repo_id, kind, topic, content, task_id, embedding, created_at, status, superseded_by, access_count
	          FROM memories ORDER BY id DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, types.NewNomosError(types.ErrCodeSubstrate, "failed querying memories: %v", err)
	}
	defer rows.Close()

	return collectMemoryRows(rows)
}

// Query filters memories by criteria.
func (s *Store) Query(filter MemoryFilter) ([]*Memory, error) {
	all, err := s.ListAll()
	if err != nil {
		return nil, err
	}

	var matched []*Memory
	for _, m := range all {
		if !matchesMemoryFilter(m, filter) {
			continue
		}
		matched = append(matched, m)
		if filter.Limit > 0 && len(matched) >= filter.Limit {
			break
		}
	}
	return matched, nil
}

func matchesMemoryFilter(m *Memory, filter MemoryFilter) bool {
	if filter.Kind != "" && m.Kind != filter.Kind {
		return false
	}
	if filter.Topic != "" && !strings.EqualFold(m.Topic, filter.Topic) {
		return false
	}
	if filter.Status != "" && m.Status != filter.Status {
		return false
	}
	if filter.TaskID != "" && !strings.EqualFold(m.TaskID, filter.TaskID) {
		return false
	}
	if !filter.IncludeCrossRepo && filter.RepoID != "" {
		clean := strings.ToLower(strings.TrimSpace(filter.RepoID))
		memRepo := strings.ToLower(strings.TrimSpace(m.RepoID))
		if memRepo != clean && memRepo != "global" && memRepo != "" {
			return false
		}
	}
	return true
}

// RecordAccess increments access count for a memory record.
func (s *Store) RecordAccess(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `UPDATE memories SET access_count = access_count + 1 WHERE id = ?`
	_, err := s.db.Exec(query, id)
	return err
}

// InitSchema ensures the memories table and indexes are created.
func (s *Store) InitSchema() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(memSchemaDDL)
	return err
}

type memScanner interface {
	Scan(dest ...any) error
}

func scanMemoryRow(scanner memScanner) (*Memory, error) {
	var (
		m            Memory
		embBytes     []byte
		createdStr   string
		kindStr      string
		statusStr    string
		supersededBy sql.NullInt64
	)
	err := scanner.Scan(&m.ID, &m.RepoID, &kindStr, &m.Topic, &m.Content, &m.TaskID,
		&embBytes, &createdStr, &statusStr, &supersededBy, &m.AccessCount)
	if err != nil {
		return nil, err
	}

	m.Kind = MemoryKind(kindStr)
	m.Status = MemoryStatus(statusStr)
	m.Embedding = DecodeVector(embBytes)
	m.SupersededBy = int(supersededBy.Int64)
	if t, pErr := time.Parse(time.RFC3339, createdStr); pErr == nil {
		m.CreatedAt = t
	}
	return &m, nil
}

func collectMemoryRows(rows *sql.Rows) ([]*Memory, error) {
	var list []*Memory
	for rows.Next() {
		m, err := scanMemoryRow(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, nil
}
