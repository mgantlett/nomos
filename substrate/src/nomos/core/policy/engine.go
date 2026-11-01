// Package policy provides deterministic CUE schema compilation, validation,
// and gate violation evaluation adhering to the CUE Singularity (ADR-0056).
package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"cuelang.org/go/encoding/yaml"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/types"
	"github.com/mgantlett/nomos/substrate/src/nomos/core/workspace"
)

// CUEEngine provides deterministic CUE schema compilation and evaluation.
type CUEEngine struct {
	ctx   *cue.Context
	cache map[string]cue.Value
	mu    sync.RWMutex
}

var (
	defaultEngine *CUEEngine
	engineOnce    sync.Once
)

// GetDefaultEngine returns the singleton CUEEngine instance.
func GetDefaultEngine() *CUEEngine {
	engineOnce.Do(func() {
		defaultEngine = NewCUEEngine()
	})
	return defaultEngine
}

// NewCUEEngine instantiates a fresh CUE engine with an isolated context.
func NewCUEEngine() *CUEEngine {
	return &CUEEngine{
		ctx:   cuecontext.New(),
		cache: make(map[string]cue.Value),
	}
}

// resolveSchemaFile locates the schema path across repository and worktree roots.
func resolveSchemaFile(relOrAbsPath string) ([]byte, error) {
	if filepath.IsAbs(relOrAbsPath) {
		return os.ReadFile(relOrAbsPath)
	}

	root, err := workspace.FindWorkingTreeRoot(".")
	if err != nil {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			root = wd
		} else {
			root = "."
		}
	}

	candidates := []string{
		filepath.Join(root, relOrAbsPath),
		filepath.Join(root, "intent", "contracts", "cue", filepath.Base(relOrAbsPath)),
		filepath.Join(root, "intent", relOrAbsPath),
	}

	for _, cand := range candidates {
		data, err := os.ReadFile(cand)
		if err == nil {
			return data, nil
		}
	}

	return nil, types.NewNomosError(types.ErrCodeCore, "unable to locate schema file: %s", relOrAbsPath)
}

// loadSchema reads or compiles a CUE schema string or file path.
func (e *CUEEngine) loadSchema(pathOrContent string) (cue.Value, error) {
	var content []byte
	if strings.HasSuffix(pathOrContent, ".cue") && !strings.Contains(pathOrContent, "\n") {
		data, err := resolveSchemaFile(pathOrContent)
		if err != nil {
			return cue.Value{}, err
		}
		content = data
	} else {
		content = []byte(pathOrContent)
	}

	schemaVal := e.ctx.CompileBytes(content)
	if err := schemaVal.Err(); err != nil {
		return cue.Value{}, types.NewNomosError(types.ErrCodeCore, "invalid CUE schema: %v", err)
	}
	return schemaVal, nil
}

// getOrLoadSchema returns a cached compiled schema or loads it.
func (e *CUEEngine) getOrLoadSchema(pathOrContent string) (cue.Value, error) {
	e.mu.RLock()
	val, ok := e.cache[pathOrContent]
	e.mu.RUnlock()
	if ok {
		return val, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if val, ok = e.cache[pathOrContent]; ok {
		return val, nil
	}

	val, err := e.loadSchema(pathOrContent)
	if err != nil {
		return cue.Value{}, err
	}
	e.cache[pathOrContent] = val
	return val, nil
}

// parseInput converts generic input structures to a concrete CUE value.
func (e *CUEEngine) parseInput(input any) (cue.Value, error) {
	switch raw := input.(type) {
	case string:
		return e.ctx.CompileString(raw), nil
	case []byte:
		expr, err := yaml.Extract("", raw)
		if err == nil {
			val := e.ctx.BuildFile(expr)
			if err := val.Err(); err == nil {
				return val, nil
			}
		}
		return e.ctx.CompileBytes(raw), nil
	default:
		bytes, err := json.Marshal(input)
		if err == nil {
			val := e.ctx.CompileBytes(bytes)
			if err := val.Err(); err == nil {
				return val, nil
			}
		}
		return e.ctx.Encode(input), nil
	}
}

// Validate unifies input against a schema and asserts concrete validity.
func (e *CUEEngine) Validate(schema string, data any) error {
	schemaVal, err := e.getOrLoadSchema(schema)
	if err != nil {
		return err
	}

	dataVal, err := e.parseInput(data)
	if err != nil {
		return err
	}
	if err := dataVal.Err(); err != nil {
		return types.NewNomosError(types.ErrCodeCore, "failed encoding input to CUE: %v", err)
	}

	unified := schemaVal.Unify(dataVal)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return types.NewNomosError(types.ErrCodeCore, "CUE schema validation failed: %v", err)
	}
	return nil
}

// Evaluate evaluates and validates data against the schema.
func (e *CUEEngine) Evaluate(schema string, data any) error {
	return e.Validate(schema, data)
}

// EvaluateBool evaluates a boolean query against a unified schema and input.
func (e *CUEEngine) EvaluateBool(ctx context.Context, schemaPath, query string, input any) (bool, error) {
	schemaVal, err := e.getOrLoadSchema(schemaPath)
	if err != nil {
		return false, err
	}

	dataVal, err := e.parseInput(input)
	if err != nil {
		return false, err
	}

	unified := schemaVal.Unify(dataVal)
	field := query
	if parts := strings.Split(query, "."); len(parts) > 0 {
		field = parts[len(parts)-1]
	}

	target := unified.LookupPath(cue.ParsePath(field))
	if !target.Exists() {
		return false, nil
	}
	return target.Bool()
}

// EvaluateViolations extracts slice of violation strings from unified schema.
func (e *CUEEngine) EvaluateViolations(ctx context.Context, schemaPath, query string, input any) ([]string, error) {
	schemaVal, err := e.getOrLoadSchema(schemaPath)
	if err != nil {
		return nil, err
	}

	dataVal, err := e.parseInput(input)
	if err != nil {
		return nil, err
	}

	unified := schemaVal.Unify(dataVal)
	field := query
	if parts := strings.Split(query, "."); len(parts) > 0 {
		field = parts[len(parts)-1]
	}

	target := unified.LookupPath(cue.ParsePath(field))
	if !target.Exists() || target.Kind() != cue.ListKind {
		return nil, nil
	}

	iter, err := target.List()
	if err != nil {
		return nil, err
	}

	var violations []string
	for iter.Next() {
		if s, err := iter.Value().String(); err == nil {
			violations = append(violations, s)
		}
	}
	return violations, nil
}

// LoadValue returns the compiled cue.Value for a schema path.
func (e *CUEEngine) LoadValue(schemaPath string) (cue.Value, error) {
	return e.getOrLoadSchema(schemaPath)
}

// ValidatePackageDir compiles and validates all CUE schemas inside a directory.
func (e *CUEEngine) ValidatePackageDir(cueDir string) error {
	cfg := &load.Config{Dir: cueDir}
	bps := load.Instances([]string{"."}, cfg)
	if len(bps) == 0 || bps[0].Err != nil {
		errDetail := ""
		if len(bps) > 0 && bps[0].Err != nil {
			errDetail = bps[0].Err.Error()
		}
		return fmt.Errorf("CUE schema compilation error: %s", errDetail)
	}

	for _, bp := range bps {
		val := e.ctx.BuildInstance(bp)
		if val.Err() != nil {
			return fmt.Errorf("CUE value compilation error: %v", val.Err())
		}
	}
	return nil
}

