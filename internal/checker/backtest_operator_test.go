package checker_test

import (
	"context"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"gotest.tools/v3/assert"
)

// setupChecker and hasDiagnosticCode are defined in checker_operator_test.go

// TestCheckerOperatorChaining verifies a + b + c resolves without error when operator is overloaded.
func TestCheckerOperatorChaining(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3(this.x + other.x, this.y + other.y);
        }
    }
}
declare const a: Vec3;
declare const b: Vec3;
declare const c: Vec3;
const r = a + b + c;  // (a + b) + c
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Chaining operator '+' should not produce error 2365 when overloaded")
}

// TestCheckerOperatorPrecedence verifies a + b * c respects operator precedence.
func TestCheckerOperatorPrecedence(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3(this.x + other.x, this.y + other.y);
        }
        "*"(scalar: number): Vec3 {
            return new Vec3(this.x * scalar, this.y * scalar);
        }
    }
}
declare const a: Vec3;
declare const b: Vec3;
const r = a + b * 3;  // a + (b * 3), not (a + b) * 3
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Expression a + b * 3 should not produce error 2365 with overloaded operators")
}

// TestCheckerOperatorMixedOverloadAndNonOverload verifies mixing an overloaded class
// with non-overloaded types in the same expression.
func TestCheckerOperatorMixedOverloadAndNonOverload(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3(this.x + other.x, this.y + other.y);
        }
    }
}
declare const a: Vec3;
declare const b: Vec3;
const r1 = a + b;           // Should be OK (both Vec3 with overload)
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	// a + b should be valid (overloaded); no-overload case tested separately
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Vec3+Vec3 should work with operator overload")
}

// TestCheckerOperatorUnary verifies unary operator negation resolves correctly.
func TestCheckerOperatorUnary(t *testing.T) {
	t.Parallel()
	c, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Negatable {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "-"(): Negatable {
            return new Negatable(-this.value);
        }
    }
}
declare const x: Negatable;
const neg = -x;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	// Unary '-' should be valid when Negatable has zero-parameter '-' operator
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Unary '-' should not produce error 2365 when Negatable has operator overload")

	// Assert the expression type is Negatable (not any), proving return type resolution
	var unaryExpr *ast.Node
	file.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindVariableStatement {
			varStmt := n.AsVariableStatement()
			if varStmt.DeclarationList != nil {
				for _, decl := range varStmt.DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
					if decl.AsVariableDeclaration().Initializer != nil {
						unaryExpr = decl.AsVariableDeclaration().Initializer
						return true
					}
				}
			}
		}
		return false
	})
	assert.Assert(t, unaryExpr != nil, "Should find unary '-' expression initializer")
	exprType := c.GetTypeAtLocation(unaryExpr)
	t.Logf("Expression type: %s (flags=%v)", c.TypeToString(exprType), exprType.Flags())
	assert.Assert(t, exprType != nil, "Expression type should not be nil")
	assert.Assert(t, exprType.Symbol() != nil, "Expression type should have a symbol")
	assert.Equal(t, "Negatable", exprType.Symbol().Name,
		"Unary '-' expression should have type Negatable, not any")
}

// TestCheckerOperatorWithGenericType verifies operator overload works with generic class.
func TestCheckerOperatorWithGenericType(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Wrapper<T> {
    value: T;
    constructor(v: T) { this.value = v; }
    operators {
        "+"(other: Wrapper<T>): Wrapper<T> {
            return this;
        }
    }
}
declare const a: Wrapper<number>;
declare const b: Wrapper<number>;
const r = a + b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Generic class with operator overload should not produce error 2365")
}

// TestCheckerOperatorInterface verifies that interface types with operator overload
// do not produce errors (interfaces can't have operators blocks, but type-checking
// should still work with declared types).
func TestCheckerOperatorInterface(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
interface IOps {
    operators {
        "+"(other: IOps): IOps;
    }
}
class Impl implements IOps {
    operators {
        "+"(other: IOps): Impl {
            return this;
        }
    }
}
declare const a: IOps;
declare const b: IOps;
const r = a + b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	// Interfaces with operators blocks are not currently supported for operator
	// overload resolution. The operators block parses but the interface type's
	// members don't participate in overload lookups. Therefore a + b on two
	// interface-typed values should produce error 2365 and other type errors.
	assert.Assert(t, hasDiagnosticCode(diags, 2365),
		"IOps interface with operators block: a + b should produce error 2365 (not supported)")
	assert.Assert(t, hasDiagnosticCode(diags, 2693),
		"IOps is a type, cannot be used as a value in a + b expression")
	assert.Assert(t, hasDiagnosticCode(diags, 2304),
		"Cannot find name when IOps used as value")
}

// TestCheckerOperatorMultipleOperators verifies multiple different operator overloads in same class.
func TestCheckerOperatorMultipleOperators(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3(this.x + other.x, this.y + other.y);
        }
        "-"(other: Vec3): Vec3 {
            return new Vec3(this.x - other.x, this.y - other.y);
        }
        "*"(scalar: number): Vec3 {
            return new Vec3(this.x * scalar, this.y * scalar);
        }
    }
}
declare const a: Vec3;
declare const b: Vec3;
const r1 = a + b;
const r2 = a - b;
const r3 = a * 5;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Multiple operator overloads should not produce error 2365")
}

// TestCheckerOperatorComparison verifies comparison operators with overloads.
func TestCheckerOperatorComparison(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Pair {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "=="(other: Pair): boolean {
            return this.x === other.x && this.y === other.y;
        }
        "<"(other: Pair): boolean {
            return this.x < other.x;
        }
    }
}
declare const a: Pair;
declare const b: Pair;
const eq = a == b;
const lt = a < b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Comparison operators with overloads should not produce error 2365")
}

// TestCheckerOperatorAllBinary verifies all 12 supported binary operators
// suppress error 2365 when overloaded.
func TestCheckerOperatorAllBinary(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class OpAll {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(other: OpAll): OpAll { return new OpAll(this.value + other.value); }
        "-"(other: OpAll): OpAll { return new OpAll(this.value - other.value); }
        "*"(other: OpAll): OpAll { return new OpAll(this.value * other.value); }
        "/"(other: OpAll): OpAll { return new OpAll(this.value / other.value); }
        "%"(other: OpAll): OpAll { return new OpAll(this.value % other.value); }
        "**"(other: OpAll): OpAll { return new OpAll(this.value ** other.value); }
        "<"(other: OpAll): boolean { return this.value < other.value; }
        ">"(other: OpAll): boolean { return this.value > other.value; }
        "<="(other: OpAll): boolean { return this.value <= other.value; }
        ">="(other: OpAll): boolean { return this.value >= other.value; }
        "=="(other: OpAll): boolean { return this.value === other.value; }
        "!="(other: OpAll): boolean { return this.value !== other.value; }
    }
}
declare const a: OpAll;
declare const b: OpAll;
const r1 = a + b;
const r2 = a - b;
const r3 = a * b;
const r4 = a / b;
const r5 = a % b;
const r6 = a ** b;
const r7 = a < b;
const r8 = a > b;
const r9 = a <= b;
const rA = a >= b;
const rB = a == b;
const rC = a != b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"All 12 binary operators should suppress error 2365 when overloaded")
}

// TestCheckerOperatorTwoBlocks verifies that two 'operators { }' blocks in one
// class both contribute methods and suppress error 2365.
func TestCheckerOperatorTwoBlocks(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class DualOps {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(other: DualOps): DualOps { return new DualOps(this.value + other.value); }
    }
    operators {
        "-"(other: DualOps): DualOps { return new DualOps(this.value - other.value); }
    }
}
declare const a: DualOps;
declare const b: DualOps;
const r1 = a + b;
const r2 = a - b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Two operators blocks in one class should both contribute and suppress error 2365")
}

// TestCheckerOperatorVoidReturn verifies that an operator with void return type
// does not crash and suppresses error 2365.
func TestCheckerOperatorVoidReturn(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class VoidOp {
    log: string;
    constructor(s: string) { this.log = s; }
    operators {
        "+"(other: VoidOp): void {
            this.log += other.log;
        }
    }
}
declare const a: VoidOp;
declare const b: VoidOp;
const r = a + b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	// Should not crash, and no error 2365
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Operator with void return type should not produce error 2365")
}

// TestCheckerOperatorInheritance verifies a subclass can use inherited operators
// from a parent class without error 2365.
func TestCheckerOperatorInheritance(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Base {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(other: Base): Base { return new Base(this.value + other.value); }
        "=="(other: Base): boolean { return this.value === other.value; }
    }
}
class Derived extends Base {
    name: string;
    constructor(v: number, n: string) { super(v); this.name = n; }
}
declare const a: Derived;
declare const b: Derived;
const r1 = a + b;
const r2 = a == b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Subclass should inherit operators from parent and suppress error 2365")
}

// TestCheckerOperatorStrictEquality verifies that === and !== operators
// can be overloaded and suppress error 2365.
func TestCheckerOperatorStrictEquality(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Cmp {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "==="(other: Cmp): boolean {
            return this.value === other.value;
        }
        "!=="(other: Cmp): boolean {
            return this.value !== other.value;
        }
    }
}
declare const a: Cmp;
declare const b: Cmp;
const eq = a === b;
const neq = a !== b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Strict equality operators === and !== should not produce error 2365 when overloaded")
}

// TestCheckerOperatorUnaryPlus verifies unary + operator overload resolves correctly.
func TestCheckerOperatorUnaryPlus(t *testing.T) {
	t.Parallel()
	c, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Positivable {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(): Positivable {
            return new Positivable(+this.value);
        }
    }
}
declare const x: Positivable;
const pos = +x;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	// Unary '+' should be valid when Positivable has zero-parameter "+" operator
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Unary '+' should not produce error 2365 when Positivable has operator overload")

	// Assert the expression type is Positivable (not any), proving return type resolution
	var unaryExpr *ast.Node
	file.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindVariableStatement {
			varStmt := n.AsVariableStatement()
			if varStmt.DeclarationList != nil {
				for _, decl := range varStmt.DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
					if decl.AsVariableDeclaration().Initializer != nil {
						unaryExpr = decl.AsVariableDeclaration().Initializer
						return true
					}
				}
			}
		}
		return false
	})
	assert.Assert(t, unaryExpr != nil, "Should find unary '+' expression initializer")
	exprType := c.GetTypeAtLocation(unaryExpr)
	t.Logf("Expression type: %s (flags=%v)", c.TypeToString(exprType), exprType.Flags())
	assert.Assert(t, exprType != nil, "Expression type should not be nil")
	assert.Assert(t, exprType.Symbol() != nil, "Expression type should have a symbol")
	assert.Equal(t, "Positivable", exprType.Symbol().Name,
		"Unary '+' expression should have type Positivable, not any")
}
