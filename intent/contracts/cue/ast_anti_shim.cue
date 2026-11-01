package schemas

// AntiShimDTO defines the AST shape extracted by the Go substrate.
#AntiShimDTO: {
	Name: string
	BodyStatementCount: int
	IsPanicShim: bool
	ForbiddenLiteralShim: string

	// Zero-Trust AST Bounds
	// 1. A function must have at least one statement in its body to not be an empty stub.
	_not_empty: true & (BodyStatementCount > 0)
	// 2. A function must not be solely composed of a panic("TODO") shim.
	_not_panic: true & (IsPanicShim == false)
	// 3. A function must not contain any forbidden literals.
	_no_forbidden_literal: true & (ForbiddenLiteralShim == "")
}

// Enforce closed-world at root
#AntiShimDTO
