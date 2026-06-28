package tstransforms

import (
	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/binder"
	"github.com/microsoft/typescript-go/internal/printer"
	"github.com/microsoft/typescript-go/internal/transformers"
)

// OperatorOverloadTransformer rewrites expressions that have operator overloads
// into element-access call expressions:
//
//	a + b  →  a["+"](b)
//	a - b  →  a["-"](b)
//	-a    →  a["-"](a)
//	+a    →  a["+"](a)
//
// It relies on the EmitResolver.GetOperatorOverload to determine if a binary or
// prefix-unary expression node has a resolved operator overload from the checker.
type OperatorOverloadTransformer struct {
	transformers.Transformer
	resolver     binder.ReferenceResolver
	emitResolver printer.EmitResolver
}

func NewOperatorOverloadTransformer(opt *transformers.TransformOptions) *transformers.Transformer {
	tx := &OperatorOverloadTransformer{resolver: opt.Resolver, emitResolver: opt.EmitResolver}
	return tx.NewTransformer(tx.visit, opt.Context)
}

func (tx *OperatorOverloadTransformer) visit(node *ast.Node) *ast.Node {
	switch node.Kind {
	case ast.KindBinaryExpression:
		return tx.visitBinaryExpression(node.AsBinaryExpression())
	case ast.KindPrefixUnaryExpression:
		return tx.visitPrefixUnaryExpression(node.AsPrefixUnaryExpression())
	default:
		return tx.Visitor().VisitEachChild(node)
	}
}

// operatorOverloadFor resolves the operator-overload name for a transformed node.
// The node handed to the transformer comes from earlier emit passes (e.g. the type
// eraser), which may have rebuilt the subtree and left its parent pointers and binder
// state inconsistent. GetOperatorOverload re-enters the checker, which requires a fully
// bound parse tree, so we resolve back to the original parse-tree node and query that.
func (tx *OperatorOverloadTransformer) operatorOverloadFor(node *ast.Node) string {
	parseNode := tx.EmitContext().ParseNode(node)
	if parseNode == nil {
		return ""
	}
	return tx.emitResolver.GetOperatorOverload(parseNode)
}

// visitBinaryExpression checks if a binary expression has a resolved operator overload
// and rewrites it as an element-access call if so.
func (tx *OperatorOverloadTransformer) visitBinaryExpression(node *ast.BinaryExpression) *ast.Node {
	// First visit children (bottom-up) so that nested transforms (e.g. a+b+c) are applied first.
	updated := tx.Visitor().VisitEachChild(node.AsNode()).AsBinaryExpression()

	// Only transform if the operator node itself was marked with an overload
	// Use the original node for the lookup since the checker stores parse-tree pointers.
	opName := tx.operatorOverloadFor(node.AsNode())
	if opName == "" {
		return updated.AsNode()
	}

	f := tx.Factory()

	// Rewrite: a + b  →  a["+"](b)
	// Build: a["opName"]
	elementAccess := f.NewElementAccessExpression(
		updated.Left,
		nil, // questionDotToken
		f.NewStringLiteral(opName, ast.TokenFlagsNone),
		ast.NodeFlagsNone,
	)

	// Build: a["opName"](b)
	call := f.NewCallExpression(
		elementAccess,
		nil, // questionDotToken
		nil, // typeArguments
		f.NewNodeList([]*ast.Node{updated.Right}),
		ast.NodeFlagsNone,
	)

	return call
}

// visitPrefixUnaryExpression checks if a prefix unary expression has a resolved operator
// overload and rewrites it as an element-access call if so.
// Prefix unary -a becomes a["-"](a), +a becomes a["+"](a).
func (tx *OperatorOverloadTransformer) visitPrefixUnaryExpression(node *ast.PrefixUnaryExpression) *ast.Node {
	// First visit children (bottom-up).
	updated := tx.Visitor().VisitEachChild(node.AsNode()).AsPrefixUnaryExpression()

	// Only transform if the operator node itself was marked with an overload.
	opName := tx.operatorOverloadFor(node.AsNode())
	if opName == "" {
		return updated.AsNode()
	}

	f := tx.Factory()

	// Rewrite: -a  →  a["-"](a)
	// The operand is both the receiver and the argument.
	// Build: operand["opName"]
	elementAccess := f.NewElementAccessExpression(
		updated.Operand,
		nil, // questionDotToken
		f.NewStringLiteral(opName, ast.TokenFlagsNone),
		ast.NodeFlagsNone,
	)

	// Build: operand["opName"](operand)
	call := f.NewCallExpression(
		elementAccess,
		nil, // questionDotToken
		nil, // typeArguments
		f.NewNodeList([]*ast.Node{updated.Operand}),
		ast.NodeFlagsNone,
	)

	return call
}
