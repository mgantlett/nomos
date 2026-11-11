package mem

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestEncodeDecodeVector(t *testing.T) {
	orig := []float32{0.1, -0.5, 0.99, -1.0}
	encoded := EncodeVector(orig)
	if len(encoded) != len(orig)*4 {
		t.Fatalf("unexpected encoded len: %d", len(encoded))
	}

	decoded := DecodeVector(encoded)
	if len(decoded) != len(orig) {
		t.Fatalf("unexpected decoded len: %d", len(decoded))
	}
	for i := range orig {
		if math.Abs(float64(orig[i]-decoded[i])) > 1e-6 {
			t.Errorf("mismatch at index %d: got %f, want %f", i, decoded[i], orig[i])
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	v1 := []float32{1.0, 0.0, 0.0}
	v2 := []float32{1.0, 0.0, 0.0}
	v3 := []float32{0.0, 1.0, 0.0}

	simSame := CosineSimilarity(v1, v2)
	if math.Abs(simSame-1.0) > 1e-5 {
		t.Errorf("expected 1.0 for identical vectors, got %f", simSame)
	}

	simOrtho := CosineSimilarity(v1, v3)
	if math.Abs(simOrtho-0.0) > 1e-5 {
		t.Errorf("expected 0.0 for orthogonal vectors, got %f", simOrtho)
	}
}

// TestGenerateDeterministicLocalVector verifies deterministic unit-length 64-dim subword feature hashing.
// @verifies TC-MEM-04 TC-MEM-15
func TestGenerateDeterministicLocalVector(t *testing.T) {
	text := "Deterministic SQLite Episodic Memory Architecture"
	v1 := GenerateDeterministicLocalVector(text, 64)
	v2 := GenerateDeterministicLocalVector(text, 64)

	if len(v1) != 64 {
		t.Fatalf("expected 64 dimensions, got %d", len(v1))
	}

	identicalHalves := true
	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatalf("expected identical deterministic vector, mismatch at %d", i)
		}
		if i < 32 && v1[i] != v1[i+32] {
			identicalHalves = false
		}
	}
	if identicalHalves {
		t.Fatal("TC-MEM-15: expected independent 64-dim projection (v[:32] != v[32:64])")
	}

	var sumSq float64
	for _, x := range v1 {
		sumSq += float64(x * x)
	}
	if math.Abs(sumSq-1.0) > 1e-4 {
		t.Errorf("expected unit length (norm=1.0), got %f", sumSq)
	}
}

// TestSubwordIdentifierNoiseAndDimensionFallback verifies camelCase/morphology matching, stop-word noise rejection, and mixed-dimension fallback.
// @verifies TC-MEM-15
func TestSubwordIdentifierNoiseAndDimensionFallback(t *testing.T) {
	qCamel := "worktree boundary validation"
	docCamel := "ValidateWorktreeBoundary blocks direct trunk commits."
	scoreCamel := CombinedScore(
		qCamel,
		GenerateDeterministicLocalVector(qCamel, DefaultEmbeddingDimension),
		docCamel,
		GenerateDeterministicLocalVector(docCamel, DefaultEmbeddingDimension),
	)
	if scoreCamel < 0.55 {
		t.Fatalf("TC-MEM-15: expected camelCase identifier match >= 0.55, got %.3f", scoreCamel)
	}

	qStem := "embedding"
	docStem := "Local embed pipeline projects subword vectors."
	scoreStem := CombinedScore(
		qStem,
		GenerateDeterministicLocalVector(qStem, DefaultEmbeddingDimension),
		docStem,
		GenerateDeterministicLocalVector(docStem, DefaultEmbeddingDimension),
	)
	if scoreStem <= 0.0 {
		t.Fatalf("TC-MEM-15: expected morphological subword stem match > 0, got %.3f", scoreStem)
	}

	noiseQuery := "the memory in the store for the agent with the hook"
	noiseDoc := "the shader in the canvas for the camera with the mesh"
	noiseScore := CombinedScore(
		noiseQuery,
		GenerateDeterministicLocalVector(noiseQuery, DefaultEmbeddingDimension),
		noiseDoc,
		GenerateDeterministicLocalVector(noiseDoc, DefaultEmbeddingDimension),
	)
	if noiseScore != 0.0 {
		t.Fatalf("TC-MEM-15: expected unrelated stop-word-heavy pair to score 0.0, got %.3f", noiseScore)
	}

	neural768 := make([]float32, 768)
	for i := range neural768 {
		neural768[i] = 0.03
	}
	offline64 := GenerateDeterministicLocalVector(qCamel, DefaultEmbeddingDimension)
	mixedScore := CombinedScore(qCamel, offline64, docCamel, neural768)
	if mixedScore < 0.55 {
		t.Fatalf("TC-MEM-15: expected mixed 64-vs-768 dimension fallback score >= 0.55, got %.3f", mixedScore)
	}

	legacy64 := make([]float32, 64)
	for i := 0; i < 32; i++ {
		val := float32(i+1) * 0.01
		legacy64[i] = val
		legacy64[i+32] = val
	}
	legacyScore := CombinedScore(qCamel, offline64, docCamel, legacy64)
	if legacyScore < 0.55 {
		t.Fatalf("TC-MEM-15: expected legacy duplicated-half 64-dim vector to re-project cleanly, got %.3f", legacyScore)
	}
}

func TestGenerateEmbedding_LocalServerMock(t *testing.T) {
	fake768 := make([]float32, 768)
	for i := range fake768 {
		fake768[i] = 0.05
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := embeddingResponse{
			Data: []embeddingItem{
				{Embedding: fake768, Index: 0},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	os.Setenv("LOCAL_EMBEDDING_URL", ts.URL)
	defer os.Unsetenv("LOCAL_EMBEDDING_URL")

	vec, err := GenerateEmbedding("test neural embedding")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 768 {
		t.Fatalf("expected 768 dimensions from local embedder, got %d", len(vec))
	}
}

func TestGenerateEmbedding_FailureFallback(t *testing.T) {
	// Point to dead port
	os.Setenv("LOCAL_EMBEDDING_URL", "http://127.0.0.1:54321/v1/embeddings")
	defer os.Unsetenv("LOCAL_EMBEDDING_URL")

	vec, err := GenerateEmbedding("fallback test")
	if err != nil {
		t.Fatalf("unexpected error on fallback: %v", err)
	}
	if len(vec) != DefaultEmbeddingDimension {
		t.Fatalf("expected fallback to %d dimensions, got %d", DefaultEmbeddingDimension, len(vec))
	}
}
