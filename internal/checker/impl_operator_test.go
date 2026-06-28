package checker_test

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
)

// setupChecker and hasDiagnosticCode are defined in checker_operator_test.go

func TestOperatorOverloadMultiply(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number;
    y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "*"(scalar: number): Vec3 {
            return new Vec3(this.x * scalar, this.y * scalar);
        }
    }
}
declare const a: Vec3;
const c = a * 5;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d category=%d key=%v", d.Code(), d.Category(), d.MessageKey())
	}

	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Operator '*' with scalar should be valid on Vec3 with operator overload")
}

func TestOperatorOverloadChainAdd(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number;
    y: number;
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
const r = a + b + c;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d category=%d key=%v", d.Code(), d.Category(), d.MessageKey())
	}

	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Chained operator '+' should be valid on Vec3 with operator overload")
}

func TestOperatorOverloadUnaryMinus(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
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
		t.Logf("  Diagnostic: code=%d category=%d key=%v", d.Code(), d.Category(), d.MessageKey())
	}

	// Unary '-' should be valid when Negatable has zero-parameter "-" operator
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Unary '-' should be valid on Negatable with operator overload")
}

func TestOperatorOverloadStaticOperators(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class MathUtil {
    static operators {
        "+"(a: number, b: number): string {
            return String(a) + String(b);
        }
    }
}
const result = MathUtil["+"](1, 2);
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d category=%d key=%v", d.Code(), d.Category(), d.MessageKey())
	}

	// Static operators block should bind without errors and the method should be callable
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Static operators should bind and not produce error 2365")
}
