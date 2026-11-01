package task

import (
	"path/filepath"
	"testing"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/storage"
)

func setupTestStore(t *testing.T) (*SQLiteTaskStore, func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_task.db")

	db, err := storage.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	store := NewSQLiteTaskStore(db)
	cleanup := func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed closing test db: %v", closeErr)
		}
	}
	return store, cleanup
}

func TestSQLiteTaskStore_CreateAndGet(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	task1 := TaskRecord{
		Key:        "SUB-101",
		Plane:      "substrate",
		SchemaType: SchemaSubstrate,
		Title:      "Initial Substrate Foundation",
		Status:     StatusTodo,
		Tier:       Tier1,
		Priority:   80,
	}

	if err := store.CreateTask(task1); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Duplicate creation must fail
	if err := store.CreateTask(task1); err == nil {
		t.Errorf("expected duplicate task creation to fail")
	}

	got, err := store.GetTask("SUB-101")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Key != task1.Key || got.Title != task1.Title || got.Status != StatusTodo {
		t.Errorf("got %+v, want %+v", got, task1)
	}

	// Non-existent task retrieval must return error
	if _, err := store.GetTask("SUB-999"); err == nil {
		t.Errorf("expected GetTask for missing key to return error")
	}
}

func TestSQLiteTaskStore_UpdateAndDone(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	task := TaskRecord{
		Key:        "INT-201",
		Plane:      "intent",
		SchemaType: SchemaIntent,
		Title:      "Intent Architecture Specification",
		Status:     StatusPlan,
		Tier:       Tier2,
		Priority:   90,
	}
	if err := store.CreateTask(task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	task.Title = "Updated Intent Architecture Specification"
	task.Status = StatusEdit
	if err := store.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}

	updated, err := store.GetTask("INT-201")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if updated.Title != task.Title || updated.Status != StatusEdit {
		t.Errorf("unexpected updated task: %+v", updated)
	}

	// Completion invariant: DONE requires CompletedAt
	task.Status = StatusDone
	if err := store.UpdateTask(task); err == nil {
		t.Errorf("expected DONE without CompletedAt to fail invariant check")
	}
	comp := "2026-10-23T20:00:00Z"
	task.CompletedAt = &comp
	if err := store.UpdateTask(task); err != nil {
		t.Fatalf("UpdateTask with CompletedAt failed: %v", err)
	}
}

func TestSQLiteTaskStore_DependenciesAndWavefront(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	t1 := TaskRecord{
		Key: "SUB-301", Plane: "substrate", SchemaType: SchemaSubstrate,
		Title: "Task One Independent", Status: StatusTodo, Tier: Tier1,
	}
	t2 := TaskRecord{
		Key: "SUB-302", Plane: "substrate", SchemaType: SchemaSubstrate,
		Title: "Task Two Dependent", Status: StatusTodo, Tier: Tier1,
	}

	if err := store.CreateTask(t1); err != nil {
		t.Fatalf("create t1 failed: %v", err)
	}
	if err := store.CreateTask(t2); err != nil {
		t.Fatalf("create t2 failed: %v", err)
	}

	if err := store.AddDependency("SUB-302", "SUB-301"); err != nil {
		t.Fatalf("AddDependency failed: %v", err)
	}

	wf, err := store.GetWavefront()
	if err != nil {
		t.Fatalf("GetWavefront failed: %v", err)
	}
	if len(wf) != 1 || wf[0].Key != "SUB-301" {
		t.Fatalf("expected wavefront containing only SUB-301, got: %+v", wf)
	}

	// Complete SUB-301
	comp := "2026-10-23T20:05:00Z"
	t1.Status = StatusDone
	t1.CompletedAt = &comp
	if err := store.UpdateTask(t1); err != nil {
		t.Fatalf("update t1 to done failed: %v", err)
	}

	wf2, err := store.GetWavefront()
	if err != nil {
		t.Fatalf("GetWavefront after t1 done failed: %v", err)
	}
	if len(wf2) != 1 || wf2[0].Key != "SUB-302" {
		t.Fatalf("expected wavefront containing SUB-302, got: %+v", wf2)
	}
}

func TestSQLiteTaskStore_LegacyNullScan(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_legacy_null.db")

	db, err := storage.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("failed closing test db: %v", closeErr)
		}
	}()

	store := NewSQLiteTaskStore(db)

	rawInsert := `INSERT INTO tasks (key, plane, title, status, created_at, updated_at)
	              VALUES ('SUB-888', 'substrate', 'Legacy V3 Task', 'DONE', '2026-10-01', '2026-10-01')`
	if _, err := db.Exec(rawInsert); err != nil {
		t.Fatalf("failed inserting legacy row: %v", err)
	}

	task, err := store.GetTask("SUB-888")
	if err != nil {
		t.Fatalf("GetTask failed on legacy row: %v", err)
	}
	if task.Key != "SUB-888" || task.Plane != "substrate" || task.Tier != Tier1 {
		t.Fatalf("unexpected scanned legacy task: %+v", task)
	}

	list := store.ListTasks()
	if len(list) != 1 || list[0].Key != "SUB-888" {
		t.Fatalf("unexpected task list: %+v", list)
	}
	if list[0].Project != "" {
		t.Errorf("expected NULL legacy project to scan as empty string, got %q", list[0].Project)
	}
}

func TestSQLiteTaskStore_ProjectRoundTripAndInspection(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	rec := TaskRecord{
		Key:        "SUB-401",
		Plane:      "substrate",
		Project:    "gsi-sov",
		SchemaType: SchemaSubstrate,
		Title:      "Sovereign Cockpit Feature",
		Status:     StatusTodo,
		Tier:       Tier1,
		Priority:   75,
	}
	if err := store.CreateTask(rec); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	got, err := store.GetTask("SUB-401")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Project != "gsi-sov" {
		t.Fatalf("expected project 'gsi-sov', got %q", got.Project)
	}

	wf, err := store.GetWavefront()
	if err != nil {
		t.Fatalf("GetWavefront failed: %v", err)
	}
	if len(wf) != 1 || wf[0].Project != "gsi-sov" {
		t.Fatalf("expected wavefront candidate project 'gsi-sov', got %+v", wf)
	}

	got.Project = "nomos"
	if err := store.UpdateTask(*got); err != nil {
		t.Fatalf("UpdateTask failed: %v", err)
	}
	reloaded, err := store.GetTask("SUB-401")
	if err != nil {
		t.Fatalf("GetTask after update failed: %v", err)
	}
	if reloaded.Project != "nomos" {
		t.Fatalf("expected updated project 'nomos', got %q", reloaded.Project)
	}

	insp := BuildInspectionRecord(*reloaded, nil, nil, "", "")
	card := FormatInspectionCard(insp)
	if insp.Project != "nomos" || len(card) == 0 {
		t.Fatalf("expected inspection card to carry project 'nomos', got %+v", insp)
	}
}
