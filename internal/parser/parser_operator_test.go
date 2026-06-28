package parser_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/testutil/parsetestutil"
	"gotest.tools/v3/assert"
)

func TestParseOperatorsDeclaration(t *testing.T) {
	source := `
class Vec3 {
	operators {
		"+"(other: Vec3): Vec3 {
			return new Vec3(this.x + other.x, this.y + other.y);
		}
		"*"(scalar: number): Vec3 {
			return new Vec3(this.x * scalar, this.y * scalar);
		}
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false /*jsx*/)
	parsetestutil.CheckDiagnostics(t, sf)

	// Find the class declaration
	var classDecl *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})

	assert.Assert(t, classDecl != nil, "Expected to find a ClassDeclaration")
	class := classDecl.AsClassDeclaration()
	t.Logf("Class name: %s", class.Name().Text())
	t.Logf("Class members count: %d", len(class.Members.Nodes))

	// Find the OperatorsDeclaration among members
	var opsDecl *ast.Node
	for _, member := range class.Members.Nodes {
		t.Logf("  Member kind: %v", member.Kind)
		if member.Kind == ast.KindOperatorsDeclaration {
			opsDecl = member
		}
	}

	assert.Assert(t, opsDecl != nil, "Expected to find an OperatorsDeclaration in class body")
	ops := opsDecl.AsOperatorsDeclaration()
	t.Logf("Operator methods count: %d", len(ops.Members.Nodes))
	assert.Equal(t, 2, len(ops.Members.Nodes), "Expected 2 operator methods")

	// Check first operator method: "+"
	op1 := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	name1 := op1.Name()
	t.Logf("Op1 name kind: %v, text: %q", name1.Kind, name1.Text())
	assert.Equal(t, ast.KindStringLiteral, name1.Kind)
	assert.Equal(t, "+", name1.Text())
	assert.Equal(t, 1, len(op1.Parameters.Nodes))

	// Check second operator method: "*"
	op2 := ops.Members.Nodes[1].AsOperatorMethodDeclaration()
	name2 := op2.Name()
	assert.Equal(t, "*", name2.Text())
	assert.Equal(t, 1, len(op2.Parameters.Nodes))
}

func TestParseOperatorsDeclarationUnary(t *testing.T) {
	source := `
class Vec3 {
	operators {
		"-"(): Vec3 {
			return new Vec3(-this.x, -this.y);
		}
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false /*jsx*/)
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
	for _, member := range class.Members.Nodes {
		if member.Kind == ast.KindOperatorsDeclaration {
			opsDecl = member
		}
	}
	assert.Assert(t, opsDecl != nil)

	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 1, len(ops.Members.Nodes))

	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	assert.Equal(t, "-", op.Name().Text())
	assert.Equal(t, 0, len(op.Parameters.Nodes), "Unary operator should have 0 parameters")
}

func TestParseOperatorsDeclarationEmpty(t *testing.T) {
	source := `
class Empty {
	operators {
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false /*jsx*/)
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
	for _, member := range class.Members.Nodes {
		if member.Kind == ast.KindOperatorsDeclaration {
			opsDecl = member
		}
	}
	assert.Assert(t, opsDecl != nil, "Should parse empty operators block")
	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 0, len(ops.Members.Nodes))
}
