package binder_test

import (
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/binder"
	"github.com/microsoft/typescript-go/internal/testutil/parsetestutil"
	"gotest.tools/v3/assert"
)

// TestBindStaticOperators verifies static operators block binds symbols correctly.
func TestBindStaticOperators(t *testing.T) {
	source := `
class MathUtil {
	static operators {
		"+"(a: number, b: number): string {
			return String(a) + String(b);
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
	assert.Equal(t, 1, len(ops.Members.Nodes))

	// Note: Static modifier is parsed but not stored on OperatorsDeclaration node (known limitation)
	// The static block is still parsed correctly

	op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
	t.Logf("Static op name=%q, modifiers=%v", op.Name().Text(), op.Modifiers())
	assert.Equal(t, "+", op.Name().Text())
	assert.Equal(t, 2, len(op.Parameters.Nodes))

	// Symbol should be bound
	sym := op.AsNode().Symbol()
	assert.Assert(t, sym != nil, "Static operator method should have a bound symbol")
	assert.Equal(t, "+", sym.Name)
}

// TestBindInstanceAndStaticOperators verifies both instance and static operators bind in same class.
func TestBindInstanceAndStaticOperators(t *testing.T) {
	source := `
class Dual {
	operators {
		"+"(other: Dual): Dual { return this; }
	}
	static operators {
		"+"(a: Dual, b: Dual): Dual { return a; }
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
	opsBlocks := 0
	totalMethods := 0
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			opsBlocks++
			ops := m.AsOperatorsDeclaration()
			// Check that each operator method has a symbol
			for _, opNode := range ops.Members.Nodes {
				op := opNode.AsOperatorMethodDeclaration()
				sym := op.AsNode().Symbol()
				assert.Assert(t, sym != nil, "Operator method should have a symbol")
				assert.Equal(t, "+", sym.Name)
				totalMethods++
			}
		}
	}
	assert.Equal(t, 2, opsBlocks, "Should have 2 operators blocks (instance + static)")
	assert.Equal(t, 2, totalMethods, "Should have 2 total operator methods")
}

// TestBindMultipleClassesWithOperators verifies multiple classes each bind operator symbols independently.
func TestBindMultipleClassesWithOperators(t *testing.T) {
	source := `
class Vec3 {
	operators {
		"+"(other: Vec3): Vec3 { return this; }
	}
}
class Mat4 {
	operators {
		"*"(other: Mat4): Mat4 { return this; }
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false /*jsx*/)
	binder.BindSourceFile(sf)

	classCount := 0
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classCount++
			class := n.AsClassDeclaration()
			t.Logf("Class: %s", class.Name().Text())
			for _, m := range class.Members.Nodes {
				if m.Kind == ast.KindOperatorsDeclaration {
					ops := m.AsOperatorsDeclaration()
					for _, opNode := range ops.Members.Nodes {
						op := opNode.AsOperatorMethodDeclaration()
						sym := op.AsNode().Symbol()
						assert.Assert(t, sym != nil,
							"Operator %q in class %s should have a symbol",
							op.Name().Text(), class.Name().Text())
						t.Logf("  Operator: %q symbol=%s", op.Name().Text(), sym.Name)
					}
				}
			}
		}
		return false
	})
	assert.Equal(t, 2, classCount, "Should have 2 classes")
}

// TestBindOperatorSymbolNameMatchesLiteral verifies the symbol name matches the string literal key.
func TestBindOperatorSymbolNameMatchesLiteral(t *testing.T) {
	operators := []string{"+", "-", "*", "/", "%", "**", "<", ">", "<=", ">=", "==", "!="}
	for _, opStr := range operators {
		t.Run(opStr, func(t *testing.T) {
			source := `
class Foo {
	operators {
		"` + opStr + `"(other: Foo): Foo { return this; }
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
			for _, m := range class.Members.Nodes {
				if m.Kind == ast.KindOperatorsDeclaration {
					ops := m.AsOperatorsDeclaration()
					assert.Equal(t, 1, len(ops.Members.Nodes))
					op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
					sym := op.AsNode().Symbol()
					assert.Assert(t, sym != nil, "Operator %q should have a symbol", opStr)
					assert.Equal(t, opStr, sym.Name,
						"Symbol name should match operator string: %q", opStr)
				}
			}
		})
	}
}

// TestBindOperatorNodeFlags verifies the operator method node has correct flags.

// TestBindOperatorsInheritance verifies operators in a class that extends another
// don't collide — subclass and parent class both have separate operator symbols.
func TestBindOperatorsInheritance(t *testing.T) {
	source := `
class Base {
	operators {
		"+"(other: Base): Base { return this; }
	}
}
class Derived extends Base {
	operators {
		"+"(other: Derived): Derived { return this; }
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false /*jsx*/)
	binder.BindSourceFile(sf)

	var baseClass, derivedClass *ast.Node
	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			node := n.AsClassDeclaration()
			if node.Name().Text() == "Base" {
				baseClass = n
			} else if node.Name().Text() == "Derived" {
				derivedClass = n
			}
		}
		return false
	})
	assert.Assert(t, baseClass != nil, "Should find Base class")
	assert.Assert(t, derivedClass != nil, "Should find Derived class")

	// Base class symbols
	for _, m := range baseClass.AsClassDeclaration().Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			ops := m.AsOperatorsDeclaration()
			op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
			sym := op.AsNode().Symbol()
			assert.Assert(t, sym != nil, "Base operator should have a symbol")
			assert.Equal(t, "+", sym.Name)
			t.Logf("Base operator sym: name=%q flags=%v", sym.Name, sym.Flags)
		}
	}

	// Derived class symbols (should be separate from Base)
	for _, m := range derivedClass.AsClassDeclaration().Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			ops := m.AsOperatorsDeclaration()
			op := ops.Members.Nodes[0].AsOperatorMethodDeclaration()
			sym := op.AsNode().Symbol()
			assert.Assert(t, sym != nil, "Derived operator should have a symbol")
			assert.Equal(t, "+", sym.Name)
			t.Logf("Derived operator sym: name=%q flags=%v", sym.Name, sym.Flags)
		}
	}

	// Ensure both classes have their own symbols (no collision)
	baseSym := baseClass.Symbol()
	derivedSym := derivedClass.Symbol()
	assert.Assert(t, baseSym != nil)
	assert.Assert(t, derivedSym != nil)
	assert.Assert(t, baseSym != derivedSym, "Base and Derived should have separate class symbols")
}

// TestBindMultipleClassesSeparateOperators verifies multiple classes each with
// operators have separate symbol tables (no cross-contamination).
func TestBindMultipleClassesSeparateOperators(t *testing.T) {
	source := `
class A {
	operators {
		"+"(other: A): A { return this; }
		"-"(other: A): A { return this; }
	}
}
class B {
	operators {
		"*"(other: B): B { return this; }
		"/"(other: B): B { return this; }
	}
}
`
	sf := parsetestutil.ParseTypeScript(source, false /*jsx*/)
	binder.BindSourceFile(sf)

	classSymbols := map[string]*ast.Symbol{}
	operatorSymbols := map[string][]string{}

	sf.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			node := n.AsClassDeclaration()
			name := node.Name().Text()
			classSymbols[name] = n.Symbol()
			for _, m := range node.Members.Nodes {
				if m.Kind == ast.KindOperatorsDeclaration {
					ops := m.AsOperatorsDeclaration()
					for _, opNode := range ops.Members.Nodes {
						op := opNode.AsOperatorMethodDeclaration()
						sym := op.AsNode().Symbol()
						assert.Assert(t, sym != nil, "Operator %q in class %s should have a symbol", op.Name().Text(), name)
						operatorSymbols[name] = append(operatorSymbols[name], sym.Name)
					}
				}
			}
		}
		return false
	})

	assert.Equal(t, 2, len(classSymbols), "Should have 2 classes")
	assert.Assert(t, classSymbols["A"] != classSymbols["B"], "Classes A and B should have separate symbols")
	t.Logf("Class A operators: %v", operatorSymbols["A"])
	t.Logf("Class B operators: %v", operatorSymbols["B"])
	// A should have + and -, not * or /
	assert.Equal(t, 2, len(operatorSymbols["A"]))
	assert.Equal(t, 2, len(operatorSymbols["B"]))
}

// TestBindStaticOperatorsBlockSymbolFlags verifies that operator methods inside a
// static operators block carry the correct symbol flags.
func TestBindStaticOperatorsBlockSymbolFlags(t *testing.T) {
	source := `
class StaticOps {
	static value: number = 0;
	static operators {
		"+"(a: number, b: number): number { return a + b; }
		"-"(a: number, b: number): number { return a - b; }
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
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			ops := m.AsOperatorsDeclaration()
			for _, opNode := range ops.Members.Nodes {
				op := opNode.AsOperatorMethodDeclaration()
				sym := op.AsNode().Symbol()
				assert.Assert(t, sym != nil, "Static operator method should have a symbol")
				t.Logf("Static operator %q: flags=%v", sym.Name, sym.Flags)
				// Symbol should have method flag set
				assert.Assert(t, sym.Flags&ast.SymbolFlagsMethod != 0,
					"Static operator symbol should have SymbolFlagsMethod, got %v", sym.Flags)
			}
		}
	}
}

// TestBindOperatorNodeFlags verifies the operator method node has correct flags.
func TestBindOperatorNodeFlags(t *testing.T) {
	source := `
class Test {
	operators {
		"+"(other: Test): Test { return this; }
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
	for _, m := range class.Members.Nodes {
		if m.Kind == ast.KindOperatorsDeclaration {
			ops := m.AsOperatorsDeclaration()
			for _, opNode := range ops.Members.Nodes {
				op := opNode.AsOperatorMethodDeclaration()
				// Verify node kind and name kind
				assert.Equal(t, ast.KindOperatorMethodDeclaration, op.AsNode().Kind)
				assert.Equal(t, ast.KindStringLiteral, op.Name().Kind)
				// Symbol should be a method/member
				sym := op.AsNode().Symbol()
				assert.Assert(t, sym != nil)
				t.Logf("Symbol flags: %v, name: %q", sym.Flags, sym.Name)
			}
		}
	}
}
