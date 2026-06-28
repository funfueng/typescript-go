package compiler_test

import (
	"context"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
)

// setupProgram creates a compiler.Program from a map of files.
func setupProgram(t *testing.T, files map[string]string) *compiler.Program {
	t.Helper()
	fs := bundled.WrapFS(vfstest.FromMap(files, false /*useCaseSensitiveFileNames*/))
	host := compiler.NewCompilerHost("/", fs, bundled.LibPath(), nil, nil)
	parsed := &tsoptions.ParsedCommandLine{
		ParsedConfig: &core.ParsedOptions{
			FileNames: []string{"/test.ts"},
			CompilerOptions: &core.CompilerOptions{
				Target: core.ScriptTargetESNext,
				Module: core.ModuleKindESNext,
			},
		},
	}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: host})
	p.BindSourceFiles()
	return p
}

// TestEmitOperatorOverloadNoCrash verifies that emitting code with operator overloads
// does not crash the compiler. The emit transform for operator overloading is not yet
// implemented, but the compiler should at least not crash when encountering operators blocks.
func TestEmitOperatorOverloadNoCrash(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
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
const c = a + b;
`,
	})

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	for _, d := range result.Diagnostics {
		t.Logf("  Emit diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	// Should not be skipped
	if result.EmitSkipped {
		t.Fatal("Emit was skipped unexpectedly")
	}

	// Operator overload transform rewrites a+b as a["+"](b)
	jsOut := emittedFiles["/test.js"]
	if !strings.Contains(jsOut, `a["+"](b)`) {
		t.Errorf("Expected JS output to contain a[\"+\"](b), got:\n%s", jsOut)
	}
}

// TestEmitMultipleOperatorOverloads verifies emit with multiple different operators.
func TestEmitMultipleOperatorOverloads(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
		"/test.ts": `
class Vec3 {
    x: number;
    y: number;
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
const sum = a + b;
const diff = a - b;
`,
	})

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	for _, d := range result.Diagnostics {
		t.Logf("  Emit diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	jsOut := emittedFiles["/test.js"]
	if !strings.Contains(jsOut, `a["+"](b)`) {
		t.Errorf("Expected JS output to contain a[\"+\"](b), got:\n%s", jsOut)
	}
	if !strings.Contains(jsOut, `a["-"](b)`) {
		t.Errorf("Expected JS output to contain a[\"-\"](b), got:\n%s", jsOut)
	}
}

// TestEmitOperatorComparison verifies emit with comparison operator overloads.
func TestEmitOperatorComparison(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
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
`,
	})

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	for _, d := range result.Diagnostics {
		t.Logf("  Emit diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	jsOut := emittedFiles["/test.js"]
	if !strings.Contains(jsOut, `a["=="](b)`) {
		t.Errorf("Expected JS output to contain a[\"==\"](b), got:\n%s", jsOut)
	}
}

// TestEmitStaticOperatorOverload verifies emit with static operators block.
func TestEmitStaticOperatorOverload(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
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

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	t.Logf("Emit result: skipped=%v diagnostics=%d files=%d",
		result.EmitSkipped, len(result.Diagnostics), len(result.EmittedFiles))
	for _, d := range result.Diagnostics {
		t.Logf("  Emit diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	jsOut := emittedFiles["/test.js"]
	t.Logf("JS output (%d bytes):\n%s", len(jsOut), jsOut)
}

// TestEmitOperatorChaining verifies emit with chained operator expressions.
func TestEmitOperatorChaining(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
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
const r = a + b + c;
`,
	})

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	for _, d := range result.Diagnostics {
		t.Logf("  Emit diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	jsOut := emittedFiles["/test.js"]
	if !strings.Contains(jsOut, `a["+"](b)["+"](c)`) {
		t.Errorf("Expected JS output to contain a[\"+\"](b)[\"+\"](c), got:\n%s", jsOut)
	}
}

// TestEmitOperatorOverloadUnary verifies that unary -a emits as a["-"](a).
func TestEmitOperatorOverloadUnary(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
		"/test.ts": `
class Negatable {
    value: number;
    constructor(value: number) { this.value = value; }
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

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	for _, d := range result.Diagnostics {
		t.Logf("  Emit diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	if result.EmitSkipped {
		t.Fatal("Emit was skipped unexpectedly")
	}

	jsOut := emittedFiles["/test.js"]
	if !strings.Contains(jsOut, `x["-"](x)`) {
		t.Errorf("Expected JS output to contain x[\"-\"](x), got:\n%s", jsOut)
	}
}

// TestEmitOperatorOutputContainsClass verifies that the emitted JS contains
// the class definition. Once operator transform is implemented, we also verify
// BinaryExpression nodes are rewritten.
func TestEmitOperatorOutputContainsClass(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
		"/test.ts": `
class Test {
    operators {
        "+"(other: Test): Test { return this; }
    }
}
`,
	})

	emittedFiles := make(map[string]string)
	p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	jsOut := emittedFiles["/test.js"]

	// Verify the class appears in output
	if !strings.Contains(jsOut, "class Test") {
		t.Error("Emitted JS should contain 'class Test'")
	}

	// The operators keyword should be elided from JS output; only methods are emitted
	if strings.Contains(jsOut, "operators") {
		t.Errorf("JS output should not contain 'operators' keyword, got:\n%s", jsOut)
	}
	// Verify the operator method body is emitted correctly
	if !strings.Contains(jsOut, `["+"](other)`) {
		t.Errorf("Expected JS output to contain operator method [\"+\"](other), got:\n%s", jsOut)
	}
}

// TestEmitOperatorStrictEquality verifies that a === b and a !== b emit as
// a["==="](b) and a["!=="](b) when the class overloads those operators.
func TestEmitOperatorStrictEquality(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
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

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	for _, d := range result.Diagnostics {
		t.Logf("  Emit diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	if result.EmitSkipped {
		t.Fatal("Emit was skipped unexpectedly")
	}

	jsOut := emittedFiles["/test.js"]
	if !strings.Contains(jsOut, `a["==="](b)`) {
		t.Errorf("Expected JS output to contain a[\"===\"](b), got:\n%s", jsOut)
	}
	if !strings.Contains(jsOut, `a["!=="](b)`) {
		t.Errorf("Expected JS output to contain a[\"!==\"](b), got:\n%s", jsOut)
	}
}

// TestEmitOperatorUnaryPlus verifies that unary +a emits as a["+"](a).
func TestEmitOperatorUnaryPlus(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	p := setupProgram(t, map[string]string{
		"/test.ts": `
class Positivable {
    value: number;
    constructor(value: number) { this.value = value; }
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

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	for _, d := range result.Diagnostics {
		t.Logf("  Emit diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	if result.EmitSkipped {
		t.Fatal("Emit was skipped unexpectedly")
	}

	jsOut := emittedFiles["/test.js"]
	if !strings.Contains(jsOut, `x["+"](x)`) {
		t.Errorf("Expected JS output to contain x[\"+\"](x), got:\n%s", jsOut)
	}
}
