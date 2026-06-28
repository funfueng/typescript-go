package checker_test

import (
	"context"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/checker"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"
	"gotest.tools/v3/assert"
)

func setupChecker(t *testing.T, files map[string]string) (*checker.Checker, func(), *compiler.Program) {
	t.Helper()
	fs := bundled.WrapFS(vfstest.FromMap(files, false /*useCaseSensitiveFileNames*/))
	host := compiler.NewCompilerHost("/", fs, bundled.LibPath(), nil, nil)
	parsed, errs := tsoptions.GetParsedCommandLineOfConfigFile("/tsconfig.json", &core.CompilerOptions{}, nil, host, nil)
	assert.Equal(t, 0, len(errs), "Expected no config errors")
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: host})
	p.BindSourceFiles()
	c, done := p.GetTypeChecker(context.Background())
	return c, done, p
}

func hasDiagnosticCode(diags []*ast.Diagnostic, code int32) bool {
	for _, d := range diags {
		if d.Code() == code {
			return true
		}
	}
	return false
}

func TestCheckerOperatorOverloadAddNoError(t *testing.T) {
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
const c = a + b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")

	// Find the class and check its members include the "+" operator
	var classDecl *ast.Node
	file.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)
	sym := classDecl.Symbol()
	assert.Assert(t, sym != nil, "Class should have a symbol")
	t.Logf("Class symbol name: %q", sym.Name)

	// Find the operator method node and its symbol
	var opMethod *ast.Node
	classDecl.ForEachChild(func(n *ast.Node) bool {
		t.Logf("  Class child kind: %v", n.Kind)
		if n.Kind == ast.KindOperatorsDeclaration {
			ops := n.AsOperatorsDeclaration()
			if len(ops.Members.Nodes) > 0 {
				opMethod = ops.Members.Nodes[0]
			}
			return true
		}
		return false
	})
	assert.Assert(t, opMethod != nil, "Should find operator method node")
	opSym := opMethod.Symbol()
	t.Logf("Operator method symbol: %v, name=%q", opSym, func() string {
		if opSym != nil {
			return opSym.Name
		}
		return "<nil>"
	}())

	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d category=%d key=%v", d.Code(), d.Category(), d.MessageKey())
	}

	// With operator overload, a + b should NOT produce error 2365
	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Operator '+' should be valid on Vec3 with operator overload")
}

func TestCheckerOperatorOverloadNoOverloadError(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number;
    y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
}
declare const a: Vec3;
declare const b: Vec3;
const c = a + b;
`,
	})
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d category=%d key=%v", d.Code(), d.Category(), d.MessageKey())
	}

	// Without operator overload, a + b on Vec3 should produce an error
	assert.Assert(t, len(diags) > 0, "Expected operator error for Vec3 without overload")
}

func TestCheckerOperatorOverloadSubtract(t *testing.T) {
	t.Parallel()
	_, done, p := setupChecker(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number;
    y: number;
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
	defer done()

	file := p.GetSourceFile("/test.ts")
	diags := p.GetSemanticDiagnostics(context.Background(), file)
	t.Logf("Semantic diagnostics count: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d category=%d key=%v", d.Code(), d.Category(), d.MessageKey())
	}

	assert.Assert(t, !hasDiagnosticCode(diags, 2365),
		"Operator '-' should be valid on Vec3 with operator overload")
}
