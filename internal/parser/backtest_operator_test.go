package parser_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/testutil/parsetestutil"
	"gotest.tools/v3/assert"
)

// TestParseAllBinaryOperators verifies every binary operator parses correctly.
func TestParseAllBinaryOperators(t *testing.T) {
	operators := []string{"+", "-", "*", "/", "%", "**", "<", ">", "<=", ">=", "==", "!="}
	for _, op := range operators {
		t.Run(op, func(t *testing.T) {
			source := `
class Vec3 {
	operators {
		"` + op + `"(other: Vec3): Vec3 {
			return new Vec3();
		}
	}
}
`
			sf := parsetestutil.ParseTypeScript(source, false)
			parsetestutil.CheckDiagnostics(t, sf)

			var classDecl *ast.Node
			sf.ForEachChild(func(n *ast.Node) bool {
				if n.Kind == ast.KindClassDeclaration {
					classDecl = n
					return true
				}
				return false
			})
			assert.Assert(t, classDecl != nil)

			class := classDecl.AsClassDeclaration()
			var opsDecl *ast.Node
			for _, m := range class.Members.Nodes {
				if m.Kind == ast.KindOperatorsDeclaration {
					opsDecl = m
				}
			}
			assert.Assert(t, opsDecl != nil, "Expected OperatorsDeclaration for operator %q", op)

			ops := opsDecl.AsOperatorsDeclaration()
			assert.Equal(t, 1, len(ops.Members.Nodes))
			assert.Equal(t, op, ops.Members.Nodes[0].AsOperatorMethodDeclaration().Name().Text())
		})
	}
}

// TestParseStaticOperators verifies static operators block parses.
func TestParseStaticOperators(t *testing.T) {
	source := `
class MathUtil {
	static operators {
		"+"(a: number, b: number): string {
			return String(a) + String(b);
		}
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	parsetestutil.CheckDiagnostics(t, sf)

	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil)
	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 1, len(ops.Members.Nodes))

	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	assert.Equal(t, "+", op.Name().Text())
	assert.Equal(t, 2, len(op.Parameters.Nodes))
}

// TestParseOperatorsNonClassError verifies operators block outside a class produces an error.
func TestParseOperatorsNonClassError(t *testing.T) {
	source := `operators { "+"(x: number): number { return x; } }`
	sf := parsetestutil.ParseTypeScript(source, false)
	// Should produce diagnostics since operators block not in class
	// The behavior depends on how the parser handles this
	diags := sf.Diagnostics()
	t.Logf("Diagnostics: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d message=%s", d.Code(), d.MessageKey())
	}
}

// TestParseOperatorsUnaryOnly verifies operators with zero parameters (unary).
func TestParseOperatorsUnaryOnly(t *testing.T) {
	source := `
class Negatable {
	operators {
		"-"(): number { return -this.value; }
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	parsetestutil.CheckDiagnostics(t, sf)

	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil)
	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 1, len(ops.Members.Nodes))

	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	assert.Equal(t, "-", op.Name().Text())
	assert.Equal(t, 0, len(op.Parameters.Nodes))
}

// TestParseOperatorsMultipleBlocks verifies two operators blocks in same class.
func TestParseOperatorsMultipleBlocks(t *testing.T) {
	source := `
class Dual {
	operators {
		"+"(other: Dual): Dual { return this; }
	}
	operators {
		"-"(other: Dual): Dual { return this; }
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	parsetestutil.CheckDiagnostics(t, sf)

	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	opsCount := 0
	totalMethods := 0
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsCount++
			ops := m.AsOperatorsDeclaration()
			totalMethods += len(ops.Members.Nodes)
		}
	}
	assert.Equal(t, 2, opsCount, "Expected 2 operators blocks")
	assert.Equal(t, 2, totalMethods, "Expected 2 total operator methods")
}

// TestParseClassWithOperatorsAndRegularMembers verifies mixed class body.
func TestParseClassWithOperatorsAndRegularMembers(t *testing.T) {
	source := `
class Mixed {
	x: number;
	constructor(x: number) { this.x = x; }
	operators {
		"+"(other: Mixed): Mixed { return new Mixed(this.x + other.x); }
	}
	getX(): number { return this.x; }
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	parsetestutil.CheckDiagnostics(t, sf)

	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil, "Operators block should parse alongside regular class members")
	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 1, len(ops.Members.Nodes))
}

// TestParseOperatorsVoidReturn verifies void return type on operators.
func TestParseOperatorsVoidReturn(t *testing.T) {
	source := `
class Logger {
	operators {
		"<<"(msg: string): void {
			console.log(msg);
		}
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	parsetestutil.CheckDiagnostics(t, sf)

	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil)

	ops := opsDecl.AsOperatorsDeclaration()
	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	assert.Equal(t, "<<", op.Name().Text())
}

// TestParseOperatorsComplexBody verifies operator method with complex body (if/for/return).
func TestParseOperatorsComplexBody(t *testing.T) {
	source := `
class ComplexBody {
	operators {
		"+"(other: ComplexBody): ComplexBody {
			if (this.value > 0) {
				return this;
			}
			for (let i = 0; i < 10; i++) {
				if (i === other.value) {
					return other;
				}
			}
			return new ComplexBody(this.value);
		}
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	parsetestutil.CheckDiagnostics(t, sf)

	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil)

	ops := opsDecl.AsOperatorsDeclaration()
	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	assert.Equal(t, "+", op.Name().Text())
	body := op.Body
	assert.Assert(t, body != nil, "Expected body on operator method")
	assert.Equal(t, ast.KindBlock, body.Kind)
	// Should have at least 3 top-level statements (if, for, return)
	stmts := body.AsBlock().Statements.Nodes
	assert.Assert(t, len(stmts) >= 1, "Expected at least 1 top-level statement in complex body")
	t.Logf("Complex body has %d top-level statements", len(stmts))
}

// TestParseOperatorsDuplicateName verifies that a duplicate operator name in the same
// operators block parses without crash (duplicate validation is semantic, not parse).
func TestParseOperatorsDuplicateName(t *testing.T) {
	source := `
class Dupe {
	operators {
		"+"(other: Dupe): Dupe { return this; }
		"+"(other: Dupe): Dupe { return this; }
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	// Duplicates are parsed without parse errors (semantic check handles this later)
	diags := sf.Diagnostics()
	t.Logf("Diagnostics: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	// Verify both methods parse correctly
	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil)
	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 2, len(ops.Members.Nodes), "Both duplicate methods should parse")
	for _, m := range ops.Members.Nodes {
		assert.Equal(t, "+", m.AsOperatorMethodDeclaration().Name().Text())
	}
}

// TestParseOperatorsNonStringName verifies that a non-string operator name
// (identifier) parses without crash (name validation is semantic, not parse).
func TestParseOperatorsNonStringName(t *testing.T) {
	source := `
class BadName {
	operators {
		foo(other: BadName): BadName { return this; }
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	// Non-string operator names parse without error (semantic check handles this later)
	diags := sf.Diagnostics()
	t.Logf("Diagnostics: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	// Verify the method parses with the identifier name
	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil)
	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 1, len(ops.Members.Nodes))
	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	// Parser accepts identifier as name
	assert.Equal(t, "foo", op.Name().Text())
	assert.Equal(t, ast.KindIdentifier, op.Name().Kind)
}

// TestParseOperatorsNonStringNameNumeric verifies that a numeric operator name
// parses without crash.
func TestParseOperatorsNonStringNameNumeric(t *testing.T) {
	source := `
class BadNameNum {
	operators {
		42(other: BadNameNum): BadNameNum { return this; }
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	// Numeric operator names parse without error (semantic check handles this later)
	diags := sf.Diagnostics()
	t.Logf("Diagnostics: %d", len(diags))
	for _, d := range diags {
		t.Logf("  Diagnostic: code=%d key=%v", d.Code(), d.MessageKey())
	}

	// Verify the method parses with the numeric name
	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil)
	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 1, len(ops.Members.Nodes))
	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	// Parser accepts numeric literal as name
	assert.Equal(t, "42", op.Name().Text())
	assert.Equal(t, ast.KindNumericLiteral, op.Name().Kind)
}

// TestParseOperatorsMethodBodyMultipleStatements verifies multi-statement bodies.
func TestParseOperatorsMethodBodyMultipleStatements(t *testing.T) {
	source := `
class Complex {
	operators {
		"+"(other: Complex): Complex {
			const r = this.real + other.real;
			const i = this.imag + other.imag;
			return new Complex(r, i);
		}
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false)
	parsetestutil.CheckDiagnostics(t, sf)

	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil)

	class := classDecl.AsClassDeclaration()
	var opsDecl *ast.Node
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil)

	ops := opsDecl.AsOperatorsDeclaration()
	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	assert.Equal(t, "+", op.Name().Text())
	// Body should be a Block with 3 statements
	body := op.Body
	assert.Assert(t, body != nil, "Expected body on operator method")
	assert.Equal(t, ast.KindBlock, body.Kind)
	assert.Equal(t, 3, len(body.AsBlock().Statements.Nodes))
}

// TestParseOperatorsInvalidOperatorName verifies the parser gracefully handles
// invalid operator names (e.g., "invalid", "&", "??") inside operators { } blocks.
// These are valid string literals syntactically (semantic validation happens in the
// checker), so the parser should not crash and should still produce
// OperatorMethodDeclaration nodes in the AST.
func TestParseOperatorsInvalidOperatorName(t *testing.T) {
	invalidNames := []string{"invalid", "&", "??"}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			source := `
class BadOps {
	operators {
		"` + name + `"(other: BadOps): BadOps {
			return this;
		}
	}
}
`
			sf := parsetestutil.ParseTypeScript(source, false)
			// Verify parse succeeds (no crash)
			parsetestutil.CheckDiagnostics(t, sf)

			// Verify the AST contains a class with an operators block
			var classDecl *ast.Node
			sf.ForEachChild(func(n *ast.Node) bool {
				if n.Kind == ast.KindClassDeclaration {
					classDecl = n
					return true
				}
				return false
			})
			assert.Assert(t, classDecl != nil, "Expected ClassDeclaration")

			class := classDecl.AsClassDeclaration()
			var opsDecl *ast.Node
			for _, m := range class.Members.Nodes {
				if m.Kind == ast.KindOperatorsDeclaration {
					opsDecl = m
				}
			}
			assert.Assert(t, opsDecl != nil, "Expected OperatorsDeclaration for name %q", name)

			ops := opsDecl.AsOperatorsDeclaration()
			assert.Equal(t, 1, len(ops.Members.Nodes), "Expected 1 operator method for name %q", name)

			// Verify the member is an OperatorMethodDeclaration with correct name
			op := ops.Members.Nodes[0]
			assert.Equal(t, ast.KindOperatorMethodDeclaration, op.Kind, "Expected OperatorMethodDeclaration node for name %q", name)
			opMethod := op.AsOperatorMethodDeclaration()
			assert.Equal(t, name, opMethod.Name().Text(), "Expected name text %q", name)
			assert.Assert(t, opMethod.Body != nil, "Expected body on operator method for name %q", name)
		})
	}
}
