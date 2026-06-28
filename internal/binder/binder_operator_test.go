package binder_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/binder"
	"github.com/microsoft/typescript-go/internal/testutil/parsetestutil"
	"gotest.tools/v3/assert"
)

func TestBindOperatorsDeclaration(t *testing.T) {
	source := `
class Vec3 {
	operators {
		"+"(other: Vec3): Vec3 {
			return new Vec3(this.x + other.x, this.y + other.y);
		}
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false /*jsx*/)
	binder.BindSourceFile(sf)

	// Find class & operators declaration
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
		t.Logf("  Member kind: %v", m.Kind)
		if m.Kind == ast.KindOperatorsDeclaration {
			opsDecl = m
		}
	}
	assert.Assert(t, opsDecl != nil, "Expected OperatorsDeclaration in class body")

	ops := opsDecl.AsOperatorsDeclaration()
	assert.Equal(t, 1, len(ops.Members.Nodes))

	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	sym := op.AsNode().Symbol()
	t.Logf("Operator method symbol: %v", sym)
	assert.Assert(t, sym != nil, "Operator method should have a bound symbol")
	assert.Equal(t, "+", sym.Name)
}

func TestBindMultipleOperators(t *testing.T) {
	source := `
class Vec3 {
	operators {
		"+"(other: Vec3): Vec3 {
			return new Vec3(this.x + other.x, this.y + other.y);
		}
		"-"(other: Vec3): Vec3 {
			return new Vec3(this.x - other.x, this.y - other.y);
		}
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false /*jsx*/)
	binder.BindSourceFile(sf)

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
	assert.Equal(t, 2, len(ops.Members.Nodes))

	for i, opNode := range ops.Members.Nodes {
		op := opNode.AsOperatorMethodDeclaration()
		sym := op.AsNode().Symbol()
		assert.Assert(t, sym != nil, "Operator method %d should have a symbol", i)
		t.Logf("Op %d: name=%q symbol=%v", i, op.Name().Text(), sym.Name)
	}
}
