// Package mem implements the Nomos Mneme episodic and vector memory system.
package mem

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultEmbeddingDimension defines the dimensional size for deterministic local vectors.
const DefaultEmbeddingDimension = 64

// ResolveLocalEmbeddingEndpoint dynamically resolves the local inference endpoint adhering to Law 5.
func ResolveLocalEmbeddingEndpoint() string {
	if endpoint := os.Getenv("LOCAL_EMBEDDING_URL"); endpoint != "" {
		return endpoint
	}
	host := os.Getenv("EMBEDDING_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("EMBEDDING_PORT")
	if port == "" {
		port = "8081"
	}
	return "http://" + host + ":" + port + "/v1/embeddings"
}

// EncodeVector serializes a float32 slice into a binary byte representation.
func EncodeVector(vec []float32) []byte {
	if len(vec) == 0 {
		return nil
	}
	buf := make([]byte, len(vec)*4)
	for i, f := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// DecodeVector deserializes a binary byte representation into a float32 slice.
func DecodeVector(data []byte) []float32 {
	if len(data) == 0 || len(data)%4 != 0 {
		return nil
	}
	count := len(data) / 4
	vec := make([]float32, count)
	for i := 0; i < count; i++ {
		bits := binary.LittleEndian.Uint32(data[i*4:])
		vec[i] = math.Float32frombits(bits)
	}
	return vec
}

// CosineSimilarity computes the cosine of the angle between two float vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		valA := float64(a[i])
		valB := float64(b[i])
		dotProduct += valA * valB
		normA += valA * valA
		normB += valB * valB
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	sim := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
	if sim > 1.0 {
		return 1.0
	}
	if sim < -1.0 {
		return -1.0
	}
	return sim
}

// LexicalScore computes word token overlap between a query and a body of text.
func LexicalScore(query, text string) float64 {
	qTokens := tokenizeWords(query)
	tTokens := tokenizeWords(text)
	if len(qTokens) == 0 || len(tTokens) == 0 {
		return 0.0
	}

	tSet := make(map[string]bool)
	for _, t := range tTokens {
		tSet[t] = true
	}

	matched := 0
	for _, q := range qTokens {
		if tSet[q] {
			matched++
		}
	}

	return float64(matched) / float64(len(qTokens))
}

func tokenizeWords(s string) []string {
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	var clean []string
	for _, w := range words {
		if len(w) > 1 {
			clean = append(clean, w)
		}
	}
	return clean
}

// CombinedScore calculates a hybrid relevance score using cosine similarity and lexical overlap.
func CombinedScore(query string, queryEmb []float32, text string, targetEmb []float32) float64 {
	lex := LexicalScore(query, text)
	if len(queryEmb) > 0 && len(targetEmb) > 0 && len(queryEmb) == len(targetEmb) {
		cos := CosineSimilarity(queryEmb, targetEmb)
		if cos > 0 {
			if lex == 0 && cos < 0.50 {
				return 0.0
			}
			return 0.75*cos + 0.25*lex
		}
	}
	return lex
}

// GenerateDeterministicLocalVector computes a normalized float32 vector from content hashes.
func GenerateDeterministicLocalVector(text string, dim int) []float32 {
	if dim <= 0 {
		dim = DefaultEmbeddingDimension
	}
	vec := make([]float32, dim)
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return vec
	}

	for _, w := range words {
		hash := sha256.Sum256([]byte(w))
		for i := 0; i < dim; i++ {
			b := hash[i%len(hash)]
			val := (float32(b)/128.0 - 1.0)
			vec[i] += val
		}
	}

	var sumSquares float64
	for _, v := range vec {
		sumSquares += float64(v * v)
	}
	if sumSquares > 0 {
		mag := float32(math.Sqrt(sumSquares))
		for i := range vec {
			vec[i] /= mag
		}
	}

	return vec
}

// GenerateEmbedding synthesizes a dense vector for text.
// If a local inference server is available (port 8081 or LOCAL_EMBEDDING_URL),
// it generates real high-dimensional neural embeddings. Otherwise, it seamlessly
// falls back to pure-Go deterministic local vector synthesis.
func GenerateEmbedding(text string) ([]float32, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, nil
	}

	if vec, err := queryLocalEmbedder(trimmed); err == nil && len(vec) > 0 {
		return vec, nil
	}

	return GenerateDeterministicLocalVector(trimmed, DefaultEmbeddingDimension), nil
}

type embeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type embeddingItem struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type embeddingResponse struct {
	Data []embeddingItem `json:"data"`
}

func queryLocalEmbedder(text string) ([]float32, error) {
	if os.Getenv("NOMOS_DISABLE_LOCAL_EMBEDDER") == "1" {
		return nil, fmt.Errorf("local embedder disabled")
	}

	endpoint := ResolveLocalEmbeddingEndpoint()

	reqBody := embeddingRequest{
		Input: []string{text},
		Model: "nomic-embed-text",
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: 500 * time.Millisecond,
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "nomos-mem/mneme")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("local embedder returned status %d", resp.StatusCode)
	}

	var embResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, err
	}

	if len(embResp.Data) == 0 || len(embResp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}

	return embResp.Data[0].Embedding, nil
}
