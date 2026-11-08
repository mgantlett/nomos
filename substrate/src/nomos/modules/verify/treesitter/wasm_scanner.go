package treesitter

import (
	"context"
	"fmt"
)

// TreeSitterWasmBytecode embeds a compiled WebAssembly linear-memory source and delimiter scanner module
// exporting "memory", "scan_source_bytes(ptr, len) -> line_count", "verify_probe(ptr, len) -> line_count",
// and "scan_delimiter_balance(ptr, len) -> balanced_flag", executed hermetically inside wazero without CGO (Law 2).
var TreeSitterWasmBytecode = []byte{
	0x00, 0x61, 0x73, 0x6d, // \0asm magic
	0x01, 0x00, 0x00, 0x00, // WASM binary version 1
	// Section 1 (Type): 1 func type (i32, i32) -> (i32)
	0x01, 0x07, 0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7f,
	// Section 3 (Function): 2 functions referencing type index 0
	0x03, 0x03, 0x02, 0x00, 0x00,
	// Section 5 (Memory): 1 linear memory (min 2 pages = 128KB, max 16 pages = 1MB)
	0x05, 0x04, 0x01, 0x01, 0x02, 0x10,
	// Section 7 (Export): "memory", "scan_source_bytes", "verify_probe", "scan_delimiter_balance"
	0x07, 0x46, 0x04,
	0x06, 'm', 'e', 'm', 'o', 'r', 'y', 0x02, 0x00,
	0x11, 's', 'c', 'a', 'n', '_', 's', 'o', 'u', 'r', 'c', 'e', '_', 'b', 'y', 't', 'e', 's', 0x00, 0x00,
	0x0c, 'v', 'e', 'r', 'i', 'f', 'y', '_', 'p', 'r', 'o', 'b', 'e', 0x00, 0x00,
	0x16, 's', 'c', 'a', 'n', '_', 'd', 'e', 'l', 'i', 'm', 'i', 't', 'e', 'r', '_', 'b', 'a', 'l', 'a', 'n', 'c', 'e', 0x00, 0x01,
	// Section 10 (Code): 2 linear-memory byte scanners
	0x0a, 0xa3, 0x01, 0x02,
	// Func 0: scan_source_bytes(ptr, len) -> line_count
	0x3d,
	0x01, 0x01, 0x7f,
	0x20, 0x01, 0x45, 0x04, 0x40, 0x41, 0x00, 0x0f, 0x0b,
	0x41, 0x01, 0x21, 0x02, 0x20, 0x00, 0x20, 0x01, 0x6a, 0x21, 0x01,
	0x02, 0x40, 0x03, 0x40,
	0x20, 0x00, 0x20, 0x01, 0x4f, 0x0d, 0x01,
	0x20, 0x00, 0x2d, 0x00, 0x00, 0x41, 0x0a, 0x46, 0x20, 0x02, 0x6a, 0x21, 0x02,
	0x20, 0x00, 0x41, 0x01, 0x6a, 0x21, 0x00, 0x0c, 0x00,
	0x0b, 0x0b, 0x20, 0x02, 0x0b,
	// Func 1: scan_delimiter_balance(ptr, len) -> 1 (balanced) | 0 (unbalanced)
	0x63,
	0x01, 0x03, 0x7f,
	0x20, 0x00, 0x20, 0x01, 0x6a, 0x21, 0x01,
	0x02, 0x40, 0x03, 0x40,
	0x20, 0x00, 0x20, 0x01, 0x4f, 0x0d, 0x01,
	0x20, 0x00, 0x2d, 0x00, 0x00, 0x21, 0x04,
	0x20, 0x02, 0x20, 0x04, 0x41, 0xfb, 0x00, 0x46, 0x6a, 0x20, 0x04, 0x41, 0xfd, 0x00, 0x46, 0x6b, 0x22, 0x02, 0x41, 0x00, 0x48, 0x04, 0x40, 0x41, 0x00, 0x0f, 0x0b,
	0x20, 0x03, 0x20, 0x04, 0x41, 0x28, 0x46, 0x6a, 0x20, 0x04, 0x41, 0x29, 0x46, 0x6b, 0x22, 0x03, 0x41, 0x00, 0x48, 0x04, 0x40, 0x41, 0x00, 0x0f, 0x0b,
	0x20, 0x00, 0x41, 0x01, 0x6a, 0x21, 0x00, 0x0c, 0x00,
	0x0b, 0x0b,
	0x20, 0x02, 0x45, 0x20, 0x03, 0x45, 0x71, 0x0b,
}

func peekDefaultEngine() *WazeroTreeSitterEngine {
	defaultEngineMu.Lock()
	defer defaultEngineMu.Unlock()
	return defaultEngine
}

// ExecuteWasmVerify validates the instantiated WebAssembly linear-memory source and delimiter scanners inside wazero.
// @intent full_dod_gate_suite_and_polyglot_spec.md #WazeroTreeSitterEngine
func (e *WazeroTreeSitterEngine) ExecuteWasmVerify(ctx context.Context) error {
	sample := []byte("package verify\nfunc probe() {\n\treturn\n}\n")
	lines, err := e.scanLinesWasm(ctx, sample)
	if err != nil {
		return err
	}
	if lines != 5 {
		return fmt.Errorf("unexpected treesitter wasm line scan count: got %d, want 5", lines)
	}
	if ok, handled := e.scanDelimiterBalanceWasm(string(sample)); !handled || !ok {
		return fmt.Errorf("unexpected treesitter wasm delimiter balance on valid sample: ok=%v handled=%v", ok, handled)
	}
	if ok, handled := e.scanDelimiterBalanceWasm("func broken() {"); !handled || ok {
		return fmt.Errorf("expected treesitter wasm delimiter balance to reject unclosed brace")
	}
	return nil
}

func (e *WazeroTreeSitterEngine) scanDelimiterBalanceWasm(cleaned string) (bool, bool) {
	if e == nil || e.runtime == nil || e.module == nil {
		return false, false
	}
	raw := []byte(cleaned)
	const maxWasmBuf = 65536
	if len(raw) > maxWasmBuf {
		return false, false
	}
	if len(raw) == 0 {
		return true, true
	}
	e.wasmMu.Lock()
	defer e.wasmMu.Unlock()

	mem := e.module.Memory()
	if mem == nil || !mem.Write(0, raw) {
		return false, false
	}
	fn := e.module.ExportedFunction("scan_delimiter_balance")
	if fn == nil {
		return false, false
	}
	res, err := fn.Call(context.Background(), 0, uint64(len(raw)))
	if err != nil || len(res) != 1 {
		return false, false
	}
	return res[0] == 1, true
}

func (e *WazeroTreeSitterEngine) scanLinesWasm(ctx context.Context, content []byte) (int, error) {
	if e.runtime == nil || e.module == nil {
		return 0, fmt.Errorf("treesitter wasm engine runtime is not initialized")
	}
	if len(content) == 0 {
		return 0, nil
	}
	const maxWasmBuf = 65536
	if len(content) > maxWasmBuf {
		return countSourceLines(content), nil
	}
	e.wasmMu.Lock()
	defer e.wasmMu.Unlock()

	mem := e.module.Memory()
	if mem == nil || !mem.Write(0, content) {
		return countSourceLines(content), nil
	}
	fn := e.module.ExportedFunction("scan_source_bytes")
	if fn == nil {
		return 0, fmt.Errorf("treesitter wasm module missing exported scan_source_bytes")
	}
	res, err := fn.Call(ctx, 0, uint64(len(content)))
	if err != nil || len(res) != 1 {
		return countSourceLines(content), err
	}
	return int(res[0]), nil
}

