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

func TestGenerateDeterministicLocalVector(t *testing.T) {
	text := "Deterministic SQLite Episodic Memory Architecture"
	v1 := GenerateDeterministicLocalVector(text, 64)
	v2 := GenerateDeterministicLocalVector(text, 64)

	if len(v1) != 64 {
		t.Fatalf("expected 64 dimensions, got %d", len(v1))
	}

	for i := range v1 {
		if v1[i] != v2[i] {
			t.Fatalf("expected identical deterministic vector, mismatch at %d", i)
		}
	}

	var sumSq float64
	for _, x := range v1 {
		sumSq += float64(x * x)
	}
	if math.Abs(sumSq-1.0) > 1e-4 {
		t.Errorf("expected unit length (norm=1.0), got %f", sumSq)
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
