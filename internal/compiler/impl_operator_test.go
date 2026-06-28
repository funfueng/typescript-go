package compiler_test

import (
	"context"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/compiler"
	"gotest.tools/v3/assert"
)

// setupProgram is defined in backtest_operator_test.go

// TestEmitOperatorOverloadTransform verifies that a+b is rewritten to a["+"](b)
// and that the operators{} block itself is elided from emitted JS output.
func TestEmitOperatorOverloadTransform(t *testing.T) {
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

	assert.Assert(t, !result.EmitSkipped, "Emit should not be skipped")
	assert.Assert(t, len(result.Diagnostics) == 0, "Expected no emit diagnostics")

	jsOut := emittedFiles["/test.js"]
	t.Logf("JS output:\n%s", jsOut)

	// Verify the operator overload transform rewrites a+b to a["+"](b)
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`), "Expected a[\"+\"](b) in emitted JS, got:\n%s", jsOut)

	// Verify operators{} block itself is NOT emitted (elided)
	assert.Assert(t, !strings.Contains(jsOut, "operators"), "operators{} block should be elided from output")
}

// TestEmitOperatorOverloadSubtract verifies a-b is rewritten to a["-"](b).
func TestEmitOperatorOverloadSubtract(t *testing.T) {
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
        "-"(other: Vec3): Vec3 {
            return new Vec3(this.x - other.x, this.y - other.y);
        }
    }
}
declare const a: Vec3;
declare const b: Vec3;
const c = a - b;
`,
	})

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	assert.Assert(t, !result.EmitSkipped, "Emit should not be skipped")

	jsOut := emittedFiles["/test.js"]
	t.Logf("JS output:\n%s", jsOut)

	assert.Assert(t, strings.Contains(jsOut, `a["-"](b)`), "Expected a[\"-\"](b) in emitted JS, got:\n%s", jsOut)
	assert.Assert(t, !strings.Contains(jsOut, "operators"), "operators{} block should be elided from output")
}

// TestEmitOperatorOverloadChaining verifies a+b+c is rewritten to a["+"](b)["+"](c).
func TestEmitOperatorOverloadChaining(t *testing.T) {
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

	assert.Assert(t, !result.EmitSkipped, "Emit should not be skipped")

	jsOut := emittedFiles["/test.js"]
	t.Logf("JS output:\n%s", jsOut)

	// (a + b) + c → a["+"](b)["+"](c)
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)["+"](c)`),
		"Expected a[\"+\"](b)[\"+\"](c) in emitted JS, got:\n%s", jsOut)
}

// TestEmitOperatorOverloadWithPlainNumber verifies that plain number addition is NOT rewritten.
func TestEmitOperatorOverloadWithPlainNumber(t *testing.T) {
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
const x = 1 + 2;
`,
	})

	emittedFiles := make(map[string]string)
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	assert.Assert(t, !result.EmitSkipped, "Emit should not be skipped")

	jsOut := emittedFiles["/test.js"]
	t.Logf("JS output:\n%s", jsOut)

	// Plain number addition (1+2) should NOT be rewritten to element-access
	assert.Assert(t, !strings.Contains(jsOut, `1["+"](2)`),
		"Plain number addition should not be rewritten, got:\n%s", jsOut)
	assert.Assert(t, strings.Contains(jsOut, "1 + 2"),
		"Expected plain 1 + 2 in emitted JS, got:\n%s", jsOut)
}

// TestEmitOperatorOverloadClassEmitted verifies the class definition with operator methods
// is emitted (the operators{} contents are elided, but the class itself is emitted).
func TestEmitOperatorOverloadClassEmitted(t *testing.T) {
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
	result := p.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	assert.Assert(t, !result.EmitSkipped, "Emit should not be skipped")

	jsOut := emittedFiles["/test.js"]
	t.Logf("JS output:\n%s", jsOut)

	// Class definition must appear
	assert.Assert(t, strings.Contains(jsOut, "class Test"), "Expected 'class Test' in output")

	// operators{} keyword should be elided
	assert.Assert(t, !strings.Contains(jsOut, "operators"),
		"'operators' keyword should be elided from output")
}
