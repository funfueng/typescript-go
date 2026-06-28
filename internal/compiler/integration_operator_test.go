package compiler_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"gotest.tools/v3/assert"
)

// emitAndGetJS is a helper that runs emit and returns the JS output string plus diagnostics.
func emitAndGetJS(t *testing.T, files map[string]string) (string, []*ast.Diagnostic) {
	t.Helper()
	emittedFiles := make(map[string]string)
	result := setupProgram(t, files).Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})
	if result.EmitSkipped {
		t.Fatal("Emit was skipped unexpectedly")
	}
	return emittedFiles["/test.js"], result.Diagnostics
}

// ---------------------------------------------------------------------------
// Binary operators — all 12 supported
// ---------------------------------------------------------------------------

func TestIntegrationOperatorAllBinaryEmit(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	tests := []struct {
		op   string
		emit string
	}{
		{"+", `a["+"](b)`},
		{"-", `a["-"](b)`},
		{"*", `a["*"](b)`},
		{"/", `a["/"](b)`},
		{"%", `a["%"](b)`},
		{"**", `a["**"](b)`},
		{"<", `a["<"](b)`},
		{">", `a[">"](b)`},
		{"<=", `a["<="](b)`},
		{">=", `a[">="](b)`},
		{"==", `a["=="](b)`},
		{"!=", `a["!="](b)`},
	}

	for _, tc := range tests {
		t.Run(tc.op, func(t *testing.T) {
			t.Parallel()
			jsOut, _ := emitAndGetJS(t, map[string]string{
				"/test.ts": `
class OpAll {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "` + tc.op + `"(other: OpAll): OpAll { return new OpAll(0); }
    }
}
declare const a: OpAll;
declare const b: OpAll;
const r = a ` + tc.op + ` b;
`,
			})
			assert.Assert(t, strings.Contains(jsOut, tc.emit),
				"[%s] expected %q in output, got:\n%s", tc.op, tc.emit, jsOut)
		})
	}
}

// ---------------------------------------------------------------------------
// Strict equality === and !==
// ---------------------------------------------------------------------------

func TestIntegrationOperatorStrictEqualityEmit(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	tests := []struct {
		op   string
		emit string
	}{
		{"===", `a["==="](b)`},
		{"!==", `a["!=="](b)`},
	}

	for _, tc := range tests {
		t.Run(tc.op, func(t *testing.T) {
			t.Parallel()
			jsOut, _ := emitAndGetJS(t, map[string]string{
				"/test.ts": `
class StrictEq {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "` + tc.op + `"(other: StrictEq): boolean { return true; }
    }
}
declare const a: StrictEq;
declare const b: StrictEq;
const r = a ` + tc.op + ` b;
`,
			})
			assert.Assert(t, strings.Contains(jsOut, tc.emit),
				"[%s] expected %q in output, got:\n%s", tc.op, tc.emit, jsOut)
		})
	}
}

// ---------------------------------------------------------------------------
// Unary operator
// ---------------------------------------------------------------------------

func TestIntegrationOperatorUnaryMinusEmit(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Negatable {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "-"(): Negatable { return new Negatable(-this.value); }
    }
}
declare const x: Negatable;
const neg = -x;
`,
	})
	assert.Assert(t, strings.Contains(jsOut, `x["-"](x)`),
		"Expected x[\"-\"](x) in output, got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Chaining
// ---------------------------------------------------------------------------

func TestIntegrationOperatorChainingEmit(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 { return new Vec3(this.x + other.x, this.y + other.y); }
    }
}
declare const a: Vec3;
declare const b: Vec3;
declare const c: Vec3;
const r = a + b + c;
`,
	})
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)["+"](c)`),
		"Expected chaining a[\"+\"](b)[\"+\"](c), got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Precedence: a + b * c where both + and * are overloaded → a["+"](b["*"](c))
// ---------------------------------------------------------------------------

func TestIntegrationOperatorPrecedenceEmit(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 { return new Vec3(this.x + other.x, this.y + other.y); }
        "*"(scalar: number): Vec3 { return new Vec3(this.x * scalar, this.y * scalar); }
    }
}
declare const a: Vec3;
declare const b: Vec3;
const r = a + b * 3;
`,
	})
	// * has higher precedence so b*3 binds first: a["+"](b["*"](3))
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b["*"](3))`),
		"Expected a[\"+\"](b[\"*\"](3)) for precedence, got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Mixed: one class with overload, one without — only overloaded one transforms
// ---------------------------------------------------------------------------

func TestIntegrationOperatorMixedOverload(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, diags := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 { return new Vec3(this.x + other.x, this.y + other.y); }
    }
}
class NoOp { value: number; constructor(v: number) { this.value = v; } }
declare const a: Vec3;
declare const b: Vec3;
const r1 = a + b;   // overloaded — transforms
`,
	})

	// a + b should transform
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`),
		"Overloaded + should still emit a[\"+\"](b), got:\n%s", jsOut)

	// Check no unexpected diagnostics
	for _, d := range diags {
		t.Logf("Diag: code=%d key=%v", d.Code(), d.MessageKey())
	}
}

// ---------------------------------------------------------------------------
// Plain numbers: 1 + 2 NOT transformed (no overload on number)
// ---------------------------------------------------------------------------

func TestIntegrationOperatorPlainNumbersNotTransformed(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 { return new Vec3(this.x + other.x, this.y + other.y); }
    }
}
const x = 1 + 2;
`,
	})
	assert.Assert(t, !strings.Contains(jsOut, `1["+"](2)`),
		"Plain number addition should NOT be rewritten, got:\n%s", jsOut)
	assert.Assert(t, strings.Contains(jsOut, "1 + 2"),
		"Expected plain 1 + 2 in output, got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Generic class: Wrapper<T> with + overload
// ---------------------------------------------------------------------------

func TestIntegrationOperatorGenericClass(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Wrapper<T> {
    value: T;
    constructor(v: T) { this.value = v; }
    operators {
        "+"(other: Wrapper<T>): Wrapper<T> { return this; }
    }
}
declare const a: Wrapper<number>;
declare const b: Wrapper<number>;
const r = a + b;
`,
	})
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`),
		"Generic class operator + should emit a[\"+\"](b), got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Inheritance: Derived extends Base, Base has operators
// ---------------------------------------------------------------------------

func TestIntegrationOperatorInheritanceEmit(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Base {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(other: Base): Base { return new Base(this.value + other.value); }
    }
}
class Derived extends Base {
    name: string;
    constructor(v: number, n: string) { super(v); this.name = n; }
}
declare const a: Derived;
declare const b: Derived;
const r = a + b;
`,
	})
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`),
		"Derived instances should use inherited operators, got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Void return
// ---------------------------------------------------------------------------

func TestIntegrationOperatorVoidReturn(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class VoidOp {
    log: string;
    constructor(s: string) { this.log = s; }
    operators {
        "+"(other: VoidOp): void { this.log += other.log; }
    }
}
declare const a: VoidOp;
declare const b: VoidOp;
const r = a + b;
`,
	})
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`),
		"Void-return operator should emit a[\"+\"](b), got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Two operators blocks in same class
// ---------------------------------------------------------------------------

func TestIntegrationOperatorTwoBlocks(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Dual {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(other: Dual): Dual { return new Dual(this.value + other.value); }
    }
    operators {
        "-"(other: Dual): Dual { return new Dual(this.value - other.value); }
    }
}
declare const a: Dual;
declare const b: Dual;
const r1 = a + b;
const r2 = a - b;
`,
	})
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`),
		"First operators block: expected a[\"+\"](b), got:\n%s", jsOut)
	assert.Assert(t, strings.Contains(jsOut, `a["-"](b)`),
		"Second operators block: expected a[\"-\"](b), got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Static operators block
// ---------------------------------------------------------------------------

func TestIntegrationOperatorStaticBlock(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class MathUtil {
    static operators {
        "+"(a: number, b: number): number { return a + b; }
    }
}
const result = MathUtil["+"](1, 2);
`,
	})
	assert.Assert(t, strings.Contains(jsOut, `MathUtil["+"](1, 2)`),
		"Static operator method should be callable in output, got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Complex body (if/for/return)
// ---------------------------------------------------------------------------

func TestIntegrationOperatorComplexBody(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Complex {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(other: Complex): Complex {
            if (this.value > 0) {
                for (let i = 0; i < this.value; i++) {
                    if (i === other.value) {
                        return other;
                    }
                }
            }
            return new Complex(this.value + other.value);
        }
    }
}
declare const a: Complex;
declare const b: Complex;
const r = a + b;
`,
	})
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`),
		"Operator with complex body should emit a[\"+\"](b), got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Two different classes, each with operators — no cross-contamination
// ---------------------------------------------------------------------------

func TestIntegrationOperatorTwoClassesNoCrossContamination(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class A {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(other: A): A { return new A(this.value + other.value); }
    }
}
class B {
    text: string;
    constructor(s: string) { this.text = s; }
    operators {
        "+"(other: B): B { return new B(this.text + other.text); }
    }
}
declare const a: A;
declare const b: B;
const r1 = a + a;
const r2 = b + b;
`,
	})
	// A's operator should not affect B, and vice versa
	assert.Assert(t, strings.Contains(jsOut, `a["+"](a)`),
		"Class A operator should emit a[\"+\"](a), got:\n%s", jsOut)
	assert.Assert(t, strings.Contains(jsOut, `b["+"](b)`),
		"Class B operator should emit b[\"+\"](b), got:\n%s", jsOut)
}

// ---------------------------------------------------------------------------
// Emit — multiple operators on same class (all together)
// ---------------------------------------------------------------------------

func TestIntegrationOperatorAllInOneClass(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, diags := emitAndGetJS(t, map[string]string{
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
        "**"(other: OpAll): OpAll { return new OpAll(0); }
        "<"(other: OpAll): boolean { return this.value < other.value; }
        ">"(other: OpAll): boolean { return this.value > other.value; }
        "<="(other: OpAll): boolean { return this.value <= other.value; }
        ">="(other: OpAll): boolean { return this.value >= other.value; }
        "=="(other: OpAll): boolean { return this.value === other.value; }
        "!="(other: OpAll): boolean { return this.value !== other.value; }
        "==="(other: OpAll): boolean { return this.value === other.value; }
        "!=="(other: OpAll): boolean { return this.value !== other.value; }
        "-"(): OpAll { return new OpAll(-this.value); }
    }
}
declare const a: OpAll;
declare const b: OpAll;
const r01 = a + b;
const r02 = a - b;
const r03 = a * b;
const r04 = a / b;
const r05 = a % b;
const r06 = a ** b;
const r07 = a < b;
const r08 = a > b;
const r09 = a <= b;
const r10 = a >= b;
const r11 = a == b;
const r12 = a != b;
const r13 = a === b;
const r14 = a !== b;
const r15 = -a;
`,
	})
	for _, d := range diags {
		t.Logf("Diag: code=%d key=%v", d.Code(), d.MessageKey())
	}

	expectedEmits := []string{
		`a["+"](b)`, `a["-"](b)`, `a["*"](b)`, `a["/"](b)`,
		`a["%"](b)`, `a["**"](b)`, `a["<"](b)`, `a[">"](b)`,
		`a["<="](b)`, `a[">="](b)`, `a["=="](b)`, `a["!="](b)`,
		`a["==="](b)`, `a["!=="](b)`, `a["-"](a)`,
	}
	for _, expected := range expectedEmits {
		assert.Assert(t, strings.Contains(jsOut, expected),
			"Expected %q in output, got:\n%s", expected, jsOut)
	}

	// operators keyword should always be elided
	assert.Assert(t, !strings.Contains(jsOut, "operators"),
		"'operators' keyword should be elided from output")
}

// ---------------------------------------------------------------------------
// No crash / no infinite loop edge cases
// ---------------------------------------------------------------------------

func TestIntegrationOperatorNoCrashEmptyOperators(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Empty {
    operators {
    }
}
`,
	})
	assert.Assert(t, strings.Contains(jsOut, "class Empty"),
		"Empty operators block should not crash, got:\n%s", jsOut)
}

func TestIntegrationOperatorNoCrashOnlyOperators(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	jsOut, _ := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class OnlyOps {
    operators {
        "+"(other: OnlyOps): OnlyOps { return this; }
    }
}
`,
	})
	assert.Assert(t, strings.Contains(jsOut, "class OnlyOps"),
		"Class with only operators should emit, got:\n%s", jsOut)
}

func TestIntegrationOperatorMultipleFiles(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	fs := bundled.WrapFS(vfstest.FromMap(map[string]string{
		"/main.ts": `
import { Vec3 } from "./vec3";
declare const a: Vec3;
declare const b: Vec3;
const r = a + b;
`,
		"/vec3.ts": `
export class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 { return new Vec3(this.x + other.x, this.y + other.y); }
    }
}
`,
	}, false))
	host := compiler.NewCompilerHost("/", fs, bundled.LibPath(), nil, nil)
	parsed := &tsoptions.ParsedCommandLine{
		ParsedConfig: &core.ParsedOptions{
			FileNames: []string{"/main.ts"},
			CompilerOptions: &core.CompilerOptions{
				Target: core.ScriptTargetESNext,
				Module: core.ModuleKindESNext,
			},
		},
	}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: host})
	p.BindSourceFiles()

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})
	assert.Assert(t, !result.EmitSkipped, "Emit should not be skipped")

	jsOut := emittedFiles["/main.js"]
	t.Logf("Main JS output:\n%s", jsOut)
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`),
		"Cross-file operator overload should emit a[\"+\"](b), got:\n%s", jsOut)
}

// =========================================================================
// Negative / Error-path tests
// =========================================================================

func TestIntegrationOperatorNoOverloadError(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}
	p := setupProgram(t, map[string]string{"/test.ts": `
class Plain {
    value: number;
    constructor(v: number) { this.value = v; }
}
declare const a: Plain;
declare const b: Plain;
const r = a + b;
`})
	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diag: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, hasDiagCode(diags, 2365), "Expected error 2365 for + on non-overloaded class")
}

func TestIntegrationOperatorUnaryNoOverloadError(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}
	p := setupProgram(t, map[string]string{"/test.ts": `
class Plain {
    value: number;
    constructor(v: number) { this.value = v; }
}
declare const x: Plain;
const neg = -x;
`})
	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	for _, d := range diags {
		t.Logf("  Diag: code=%d key=%v", d.Code(), d.MessageKey())
	}
	// Unary - on a non-overloaded class should NOT produce error 2365.
	// In JS, unary - works on any type (coerces to number/NaN).
	// Only binary operators on incompatible types produce 2365.
	assert.Assert(t, !hasDiagCode(diags, 2365), "Unary - on non-overloaded class should not produce error 2365")
	// The expression still emits as-is (no transform) without crashing
	js, _ := emitAndGetJS(t, map[string]string{"/test.ts": `
class Plain {
    value: number;
    constructor(v: number) { this.value = v; }
}
declare const x: Plain;
const neg = -x;
`})
	assert.Assert(t, strings.Contains(js, "-x"), "Plain -x should emit as-is")
}

func TestIntegrationOperatorMixedBothNoOverload(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}
	p := setupProgram(t, map[string]string{"/test.ts": `
class A { value: number; constructor(v: number) { this.value = v; } }
class B { value: number; constructor(v: number) { this.value = v; } }
declare const a: A;
declare const b: B;
const r = a + b;
`})
	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	for _, d := range diags {
		t.Logf("  Diag: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, hasDiagCode(diags, 2365), "Expected error 2365 for + on two non-overloaded classes")
}

func TestIntegrationOperatorPartialOverload(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}
	p := setupProgram(t, map[string]string{"/test.ts": `
class PartialOps {
    value: number;
    constructor(v: number) { this.value = v; }
    operators {
        "+"(other: PartialOps): PartialOps { return new PartialOps(this.value + other.value); }
    }
}
declare const a: PartialOps;
declare const b: PartialOps;
const r1 = a + b;
const r2 = a * b;
`})
	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	for _, d := range diags {
		t.Logf("  Diag: code=%d key=%v", d.Code(), d.MessageKey())
	}
	assert.Assert(t, hasDiagCode(diags, 2362) || hasDiagCode(diags, 2365), "Expected error for * (not overloaded)")
	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})
	if !result.EmitSkipped {
		jsOut := emittedFiles["/test.js"]
		assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`), "Overloaded + should still emit a[\"+\"](b), got:\n%s", jsOut)
	}
}

// =========================================================================
// Type assertion tests
// =========================================================================

func TestIntegrationOperatorReturnType(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}
	p := setupProgram(t, map[string]string{"/test.ts": `
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
const c = a + b;
`})
	chk, done := p.GetTypeChecker(context.Background())
	defer done()
	file := p.GetSourceFile("/test.ts")
	var initExpr *ast.Node
	file.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindVariableStatement {
			for _, decl := range n.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				d := decl.AsVariableDeclaration()
				if d.Name().Text() == "c" && d.Initializer != nil {
					initExpr = d.Initializer
					return true
				}
			}
		}
		return false
	})
	assert.Assert(t, initExpr != nil, "Should find initializer for variable c")
	exprType := chk.GetTypeAtLocation(initExpr)
	t.Logf("Expression type: %s", chk.TypeToString(exprType))
	assert.Assert(t, exprType != nil && exprType.Symbol() != nil, "Expression type should have a symbol")
	assert.Equal(t, "Vec3", exprType.Symbol().Name, "a+b where + returns Vec3 should have type Vec3")
}

func TestIntegrationOperatorReturnTypeBoolean(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}
	p := setupProgram(t, map[string]string{"/test.ts": `
class Pair {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "=="(other: Pair): boolean {
            return this.x === other.x && this.y === other.y;
        }
    }
}
declare const a: Pair;
declare const b: Pair;
const eq = a == b;
`})
	chk, done := p.GetTypeChecker(context.Background())
	defer done()
	file := p.GetSourceFile("/test.ts")
	var initExpr *ast.Node
	file.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindVariableStatement {
			for _, decl := range n.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				d := decl.AsVariableDeclaration()
				if d.Name().Text() == "eq" && d.Initializer != nil {
					initExpr = d.Initializer
					return true
				}
			}
		}
		return false
	})
	assert.Assert(t, initExpr != nil, "Should find initializer for variable eq")
	exprType := chk.GetTypeAtLocation(initExpr)
	typeName := chk.TypeToString(exprType)
	t.Logf("Expression type: %s", typeName)
	assert.Assert(t, typeName == "boolean", "a==b where == returns boolean should have type boolean, got %q", typeName)
}

// =========================================================================
// Advanced edge case tests
// =========================================================================

func TestIntegrationOperatorDeepChaining(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}
	jsOut, _ := emitAndGetJS(t, map[string]string{"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 { return new Vec3(this.x + other.x, this.y + other.y); }
    }
}
declare const a: Vec3;
declare const b: Vec3;
declare const c: Vec3;
declare const d: Vec3;
const r = a + b + c + d;
`})
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)["+"](c)["+"](d)`),
		"4-deep chain should emit a[\"+\"](b)[\"+\"](c)[\"+\"](d), got:\n%s", jsOut)
}

func TestIntegrationOperatorNestedExpressions(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}
	jsOut, _ := emitAndGetJS(t, map[string]string{"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 { return new Vec3(this.x + other.x, this.y + other.y); }
        "*"(scalar: number): Vec3 { return new Vec3(this.x * scalar, this.y * scalar); }
    }
}
declare const a: Vec3;
declare const b: Vec3;
const r = (a + b) * 3;
`})
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`),
		"(a+b)*3 should contain a[\"+\"](b), got:\n%s", jsOut)
	assert.Assert(t, strings.Contains(jsOut, `["*"](3)`),
		"(a+b)*3 should contain [\"*\"](3), got:\n%s", jsOut)
}

// =========================================================================
// Runtime execution tests (Node.js)
// =========================================================================

// TestIntegrationOperatorVec3Runtime creates a full Vec3 class with operator
// overloading, compiles it, executes the emitted JS with Node.js, and asserts
// the runtime results are correct.
func TestIntegrationOperatorVec3Runtime(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	// Vec3 class with: + (addition), - (negation + subtraction), * (scalar multiply)
	jsOut, diags := emitAndGetJS(t, map[string]string{
		"/test.ts": `
class Vec3 {
    x: number;
    y: number;
    z: number;
    constructor(x: number, y: number, z: number) {
        this.x = x;
        this.y = y;
        this.z = z;
    }
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3(this.x + other.x, this.y + other.y, this.z + other.z);
        }
        "-"(other: Vec3): Vec3 {
            // The compiler rewrites both -a and a-b to a["-"](b).
            // For unary negation (-a), the compiler emits a["-"](a),
            // so the argument equals this.
            if (other === this) {
                return new Vec3(-this.x, -this.y, -this.z);
            }
            return new Vec3(this.x - other.x, this.y - other.y, this.z - other.z);
        }
        "*"(scalar: number): Vec3 {
            return new Vec3(this.x * scalar, this.y * scalar, this.z * scalar);
        }
    }
}

const a = new Vec3(1, 2, 3);
const b = new Vec3(4, 5, 6);

// Perform Vec3 arithmetic with overloaded operators
const sum = a + b;          //  (1+4, 2+5, 3+6) = (5,7,9)
const diff = a - b;         //  (1-4, 2-5, 3-6) = (-3,-3,-3)
const neg = -a;             //  (-1,-2,-3)
const scaled = a * 2;       //  (2,4,6)

// Store results as plain objects vector form and serialize
const results = [
    { label: "sum",    x: sum.x,    y: sum.y,    z: sum.z    },
    { label: "diff",   x: diff.x,   y: diff.y,   z: diff.z   },
    { label: "neg",    x: neg.x,    y: neg.y,    z: neg.z    },
    { label: "scaled", x: scaled.x, y: scaled.y, z: scaled.z },
];
console.log(JSON.stringify(results));
`,
	})
	for _, d := range diags {
		t.Logf("Diag: code=%d key=%v", d.Code(), d.MessageKey())
	}

	t.Logf("=== Emitted JS ===\n%s\n=== End JS ===", jsOut)

	// Run emitted JS with Node.js
	cmd := exec.Command("node", "-e", jsOut)
	output, err := cmd.CombinedOutput()
	assert.NilError(t, err, "node execution failed: %s", output)

	// Parse JSON output
	var results []map[string]interface{}
	assert.NilError(t, json.Unmarshal(output, &results), "failed to parse JSON: %s", output)

	t.Logf("Runtime results: %v", results)

	// Lookup helper
	get := func(label string) map[string]interface{} {
		for _, r := range results {
			if r["label"] == label {
				return r
			}
		}
		t.Fatalf("result not found: %s", label)
		return nil
	}

	// Assert correct Vec3 arithmetic
	// sum: (1+4, 2+5, 3+6) = (5, 7, 9)
	sumRes := get("sum")
	assert.Equal(t, 5.0, sumRes["x"])
	assert.Equal(t, 7.0, sumRes["y"])
	assert.Equal(t, 9.0, sumRes["z"])

	// diff: (1-4, 2-5, 3-6) = (-3, -3, -3)
	diffRes := get("diff")
	assert.Equal(t, -3.0, diffRes["x"])
	assert.Equal(t, -3.0, diffRes["y"])
	assert.Equal(t, -3.0, diffRes["z"])

	// neg: (-1, -2, -3)
	negRes := get("neg")
	assert.Equal(t, -1.0, negRes["x"])
	assert.Equal(t, -2.0, negRes["y"])
	assert.Equal(t, -3.0, negRes["z"])

	// scaled: (1*2, 2*2, 3*2) = (2, 4, 6)
	scaledRes := get("scaled")
	assert.Equal(t, 2.0, scaledRes["x"])
	assert.Equal(t, 4.0, scaledRes["y"])
	assert.Equal(t, 6.0, scaledRes["z"])
}

// =========================================================================
// Helpers
// =========================================================================

func hasDiagCode(diags []*ast.Diagnostic, code int32) bool {
	for _, d := range diags {
		if d.Code() == code {
			return true
		}
	}
	return false
}

func hasAnyDiagCode(diags []*ast.Diagnostic, codes ...int32) bool {
	for _, d := range diags {
		for _, c := range codes {
			if d.Code() == c {
				return true
			}
		}
	}
	return false
}
