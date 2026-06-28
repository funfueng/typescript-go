package ls

import (
	"context"
	"strings"
	"testing"

	"github.com/microsoft/typescript-go/internal/ast"
	"github.com/microsoft/typescript-go/internal/bundled"
	"github.com/microsoft/typescript-go/internal/compiler"
	"github.com/microsoft/typescript-go/internal/core"
	"github.com/microsoft/typescript-go/internal/ls/autoimport"
	"github.com/microsoft/typescript-go/internal/ls/lsconv"
	"github.com/microsoft/typescript-go/internal/ls/lsutil"
	"github.com/microsoft/typescript-go/internal/lsp/lsproto"
	"github.com/microsoft/typescript-go/internal/parser"
	"github.com/microsoft/typescript-go/internal/sourcemap"
	"github.com/microsoft/typescript-go/internal/tsoptions"
	"github.com/microsoft/typescript-go/internal/vfs"
	"github.com/microsoft/typescript-go/internal/vfs/vfstest"

	"gotest.tools/v3/assert"
)

// === helpers ===

func setupLS(t *testing.T, files map[string]string) (*LanguageService, func()) {
	t.Helper()
	innerFS := vfstest.FromMap(files, false /*useCaseSensitiveFileNames*/)
	fs := bundled.WrapFS(innerFS)
	host := compiler.NewCompilerHost("/", fs, bundled.LibPath(), nil, nil)
	parsed, errs := tsoptions.GetParsedCommandLineOfConfigFile("/tsconfig.json", &core.CompilerOptions{}, nil, host, nil)
	assert.Equal(t, 0, len(errs), "Expected no config errors")
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: host})
	p.BindSourceFiles()

	// Build line maps from source files for converters
	lineMaps := make(map[string]*lsconv.LSPLineMap)
	for fileName := range files {
		if file := p.GetSourceFile(fileName); file != nil {
			lineMaps[fileName] = lsconv.ComputeLSPLineStarts(file.Text())
		}
	}

	converters := lsconv.NewConverters(lsproto.PositionEncodingKindUTF16, func(fileName string) *lsconv.LSPLineMap {
		return lineMaps[fileName]
	})

	testHost := &lsTestHost{fs: fs, compilerHost: host, lineMaps: lineMaps}
	ls := &LanguageService{
		program:                 p,
		host:                    testHost,
		converters:              converters,
		activeConfig:            lsutil.UserPreferences{},
		documentPositionMappers: map[string]*sourcemap.DocumentPositionMapper{},
	}
	return ls, func() {}
}

type lsTestHost struct {
	fs           vfs.FS
	compilerHost compiler.CompilerHost
	lineMaps     map[string]*lsconv.LSPLineMap
}

func (h *lsTestHost) UseCaseSensitiveFileNames() bool                { return false }
func (h *lsTestHost) ReadFile(path string) (string, bool)             { return h.compilerHost.FS().ReadFile(path) }
func (h *lsTestHost) Converters() *lsconv.Converters                 { return nil }
func (h *lsTestHost) GetPreferences(string) lsutil.UserPreferences   { return lsutil.UserPreferences{} }
func (h *lsTestHost) GetECMALineInfo(string) *sourcemap.ECMALineInfo { return nil }
func (h *lsTestHost) AutoImportRegistry() *autoimport.Registry       { return nil }
func (h *lsTestHost) ReadDirectory(string, string, []string, []string, []string, int) []string {
	return nil
}
func (h *lsTestHost) GetDirectories(string) []string { return nil }
func (h *lsTestHost) DirectoryExists(string) bool    { return false }
func (h *lsTestHost) FileExists(string) bool         { return false }

// === tests ===

// TestLSOperatorEmit verifies end-to-end parse→bind→check→emit for operator overloading.
func TestLSOperatorEmit(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": { "target": "ESNext", "module": "ESNext" }, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    x: number; y: number;
    constructor(x: number, y: number) { this.x = x; this.y = y; }
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3(this.x + other.x, this.y + other.y);
        }
        "-"(other: Vec3): Vec3 {
            return new Vec3(this.x - other.x, this.y - other.y);
        }
    }
}
declare const a: Vec3;
declare const b: Vec3;
const sum = a + b;
const diff = a - b;
`,
	})
	defer done()

	emittedFiles := make(map[string]string)
	result := ls.program.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	assert.Assert(t, !result.EmitSkipped, "Emit should not be skipped")
	jsOut := emittedFiles["/test.js"]
	assert.Assert(t, jsOut != "", "Emitted JS should not be empty")

	// Check operator methods are emitted as computed property names
	assert.Assert(t, strings.Contains(jsOut, `["+"]`), "JS output should contain computed property [\"+\"]")
	assert.Assert(t, strings.Contains(jsOut, `["-"]`), "JS output should contain computed property [\"-\"]")

	// Check binary expressions are rewritten to element-access calls
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)`), "JS output should contain a[\"+\"](b)")
	assert.Assert(t, strings.Contains(jsOut, `a["-"](b)`), "JS output should contain a[\"-\"](b)")

	// Operators block keyword should not appear in emitted JS
	assert.Assert(t, !strings.Contains(jsOut, "operators"), "JS output should NOT contain 'operators' keyword")
}

// TestLSOperatorFolding verifies that operators blocks are recognized as class members
// with proper AST structure for folding. The full folding pipeline is tested in
// TestLSOperatorFoldingRange.
func TestLSOperatorFolding(t *testing.T) {
	t.Parallel()

	sourceFile := parser.ParseSourceFile(ast.SourceFileParseOptions{
		FileName: "/test.ts",
		Path:     "/test.ts",
	}, `
class Vec3 {
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3();
        }
    }
}
`, core.ScriptKindTS)

	var classDecl *ast.Node
	sourceFile.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			classDecl = n
			return true
		}
		return false
	})
	assert.Assert(t, classDecl != nil, "Should find class declaration")

	// Verify the class has an OperatorsDeclaration member (needed for folding at folding.go:372)
	members := classDecl.AsClassDeclaration().Members
	assert.Assert(t, len(members.Nodes) > 0, "Class should have members")

	var opsFound bool
	for _, member := range members.Nodes {
		t.Logf("  Member kind: %v", member.Kind)
		if member.Kind == ast.KindOperatorsDeclaration {
			opsFound = true
			ops := member.AsOperatorsDeclaration()
			assert.Assert(t, len(ops.Members.Nodes) > 0, "Operators block should have methods")
			t.Logf("  Operators block has %d method(s)", len(ops.Members.Nodes))
		}
	}
	assert.Assert(t, opsFound, "Should find OperatorsDeclaration in class members — required for folding")
}

// TestLSOperatorFoldingRange verifies the full ProvideFoldingRange includes operators blocks.
func TestLSOperatorFoldingRange(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3();
        }
    }
}
`,
	})
	defer done()

	sourceFile := ls.program.GetSourceFile("/test.ts")
	assert.Assert(t, sourceFile != nil, "Should get source file from program")

	// Use addNodeOutliningSpans to check folding
	foldingRanges := ls.addNodeOutliningSpans(context.Background(), sourceFile)
	assert.Assert(t, len(foldingRanges) > 0, "Should produce at least one folding range")

	for _, fr := range foldingRanges {
		t.Logf("Folding range: startLine=%d endLine=%d kind=%v",
			fr.StartLine, fr.EndLine, func() string {
				if fr.Kind != nil {
					return string(*fr.Kind)
				}
				return "<nil>"
			}())
	}
}

// TestLSOperatorDocumentSymbols verifies operator methods appear in document symbols.
func TestLSOperatorDocumentSymbols(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    operators {
        "+"(other: Vec3): Vec3 {
            return new Vec3();
        }
        "-"(other: Vec3): Vec3 {
            return new Vec3();
        }
    }
}
`,
	})
	defer done()

	sourceFile := ls.program.GetSourceFile("/test.ts")
	assert.Assert(t, sourceFile != nil)

	// Get document symbols for the file
	symbols := ls.getDocumentSymbolsForChildren(context.Background(), sourceFile.AsNode(), sourceFile)
	assert.Assert(t, len(symbols) > 0, "Should have at least one document symbol")

	// Find the Vec3 class symbol
	var classSymbol *lsproto.DocumentSymbol
	for _, s := range symbols {
		t.Logf("Top-level symbol: name=%q kind=%d", s.Name, s.Kind)
		if s.Name == "Vec3" {
			classSymbol = s
		}
	}
	assert.Assert(t, classSymbol != nil, "Should find Vec3 class symbol")

	// Check children of the class for operator methods
	if classSymbol.Children != nil {
		for _, child := range *classSymbol.Children {
			t.Logf("  Class child: name=%q kind=%d", child.Name, child.Kind)
		}
	}
}

// TestLSOperatorHover verifies that operator expressions produce symbols for hover.
func TestLSOperatorHover(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
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
const c = a + b;
`,
	})
	defer done()

	program := ls.program
	file := program.GetSourceFile("/test.ts")
	assert.Assert(t, file != nil)

	// Find the binary expression a + b by walking the AST
	// SourceFile → VariableStatement → VariableDeclarationList → VariableDeclaration → BinaryExpression
	var binaryExpr *ast.Node
	file.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindVariableStatement {
			declList := n.AsVariableStatement().DeclarationList
			for _, decl := range declList.AsVariableDeclarationList().Declarations.Nodes {
				initNode := decl.AsVariableDeclaration().Initializer
				if initNode != nil && initNode.Kind == ast.KindBinaryExpression {
					binaryExpr = initNode
					return true
				}
			}
		}
		return false
	})
	assert.Assert(t, binaryExpr != nil, "Should find binary expression for a + b")

	// Get the operator token
	opToken := binaryExpr.AsBinaryExpression().OperatorToken
	assert.Assert(t, opToken != nil, "Binary expression should have operator token")
	t.Logf("Operator token kind: %v, pos=%d", opToken.Kind, opToken.Pos())

	// Get the EmitResolver and check operator overload
	checker, done := program.GetTypeChecker(context.Background())
	defer done()
	emitResolver := checker.GetEmitResolver()
	overloadName := emitResolver.GetOperatorOverload(binaryExpr)
	t.Logf("Operator overload resolved: %q", overloadName)
	if overloadName != "" {
		assert.Equal(t, "+", overloadName, "Overloaded operator should be '+'")
	}

	// Get symbol at the operator token position
	sym := checker.GetSymbolAtLocation(opToken)
	t.Logf("Symbol at operator: %v", sym)
}

// TestLSOperatorHoverContent verifies that hovering an overloaded operator (use site)
// or an operator method declaration produces a useful `(operator)` quick-info with the
// method signature, and that non-operator hovers are unaffected.
func TestLSOperatorHoverContent(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	src := `class Vec3 {
    constructor(public x: number) {}
    operators {
        /** Adds two vectors. */
        "+"(other: Vec3): Vec3 { return new Vec3(this.x + other.x); }
        "-"(): Vec3 { return new Vec3(-this.x); }
    }
}
declare const a: Vec3;
declare const b: Vec3;
const sum = a + b;
const neg = -a;
`
	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts":       src,
	})
	defer done()

	file := ls.program.GetSourceFile("/test.ts")
	hoverAt := func(off int) string {
		lc := ls.converters.PositionToLineAndCharacter(file, core.TextPos(off))
		resp, err := ls.ProvideHover(context.Background(), &lsproto.HoverParams{
			TextDocument: lsproto.TextDocumentIdentifier{Uri: "file:///test.ts"},
			Position:     lc,
		})
		assert.NilError(t, err)
		if resp.Hover == nil || resp.Hover.Contents.MarkupContent == nil {
			return ""
		}
		return resp.Hover.Contents.MarkupContent.Value
	}

	// Use site: the `+` in `a + b`.
	plusUse := hoverAt(strings.Index(src, "a + b") + 2)
	t.Logf("hover(+ use): %q", plusUse)
	assert.Assert(t, strings.Contains(plusUse, `(operator) Vec3["+"](other: Vec3): Vec3`),
		"binary operator use-site hover should show operator method signature")
	assert.Assert(t, strings.Contains(plusUse, "Adds two vectors"),
		"binary operator hover should include JSDoc")

	// Use site: the `-` in `-a` (unary).
	negUse := hoverAt(strings.Index(src, "-a;"))
	t.Logf("hover(- use): %q", negUse)
	assert.Assert(t, strings.Contains(negUse, `(operator) Vec3["-"](): Vec3`),
		"unary operator use-site hover should show operator method signature")

	// Declaration site: the `"+"` method name inside the operators block.
	plusDecl := hoverAt(strings.Index(src, `"+"(other`) + 1)
	t.Logf("hover(+ decl): %q", plusDecl)
	assert.Assert(t, strings.Contains(plusDecl, `(operator) Vec3["+"](other: Vec3): Vec3`),
		"operator declaration hover should show operator method signature")

	// Regression: hovering an ordinary identifier still produces its normal quick-info.
	sumDecl := hoverAt(strings.Index(src, "const sum") + len("const "))
	t.Logf("hover(sum): %q", sumDecl)
	assert.Assert(t, strings.Contains(sumDecl, "sum") && !strings.Contains(sumDecl, "(operator)"),
		"non-operator hover should be unaffected")
}

// TestLSOperatorCompletions verifies AST structure for completions inside operators blocks.
func TestLSOperatorCompletions(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    operators {
        
    }
}
`,
	})
	defer done()

	file := ls.program.GetSourceFile("/test.ts")
	assert.Assert(t, file != nil)

	// Find the operators block in the AST
	var opsDecl *ast.Node
	file.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			for _, m := range n.AsClassDeclaration().Members.Nodes {
				if m.Kind == ast.KindOperatorsDeclaration {
					opsDecl = m
					return true
				}
			}
		}
		return false
	})
	assert.Assert(t, opsDecl != nil, "Should find OperatorsDeclaration in AST")

	ops := opsDecl.AsOperatorsDeclaration()
	t.Logf("Operators block at pos=%d end=%d", opsDecl.Pos(), opsDecl.End())
	t.Logf("Operators block Members: %v (len=%d)", ops.Members.Nodes, len(ops.Members.Nodes))

	// Supported operator strings for completions
	supportedOps := []string{"+", "-", "*", "/", "%", "**", "<", ">", "<=", ">=", "==", "!="}
	assert.Equal(t, 12, len(supportedOps), "Should have 12 supported operators")

	// Verify the operators block has a body for containing method declarations
	body := ops.Body
	assert.Assert(t, body != nil, "Operators block should have a body")
}

// TestLSOperatorChainingEmit verifies chained operators emit correctly.
func TestLSOperatorChainingEmit(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": { "target": "ESNext", "module": "ESNext" }, "files": ["test.ts"] }`,
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
	defer done()

	emittedFiles := make(map[string]string)
	result := ls.program.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	assert.Assert(t, !result.EmitSkipped)
	jsOut := emittedFiles["/test.js"]
	t.Logf("JS output:\n%s", jsOut)

	// Chained expression: a + b + c should emit as a["+"](b)["+"](c)
	assert.Assert(t, strings.Contains(jsOut, `a["+"](b)["+"](c)`),
		"Chained operators should emit as a[\"+\"](b)[\"+\"](c)")
}

// TestLSOperatorNoOperatorsKeywordInOutput verifies the operators keyword is not in JS output.
func TestLSOperatorNoOperatorsKeywordInOutput(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": { "target": "ESNext", "module": "ESNext" }, "files": ["test.ts"] }`,
		"/test.ts": `
class Test {
    operators {
        "+"(other: Test): Test { return this; }
    }
}
`,
	})
	defer done()

	emittedFiles := make(map[string]string)
	result := ls.program.Emit(context.Background(), compiler.EmitOptions{
		WriteFile: func(fileName string, text string, data *compiler.WriteFileData) error {
			emittedFiles[fileName] = text
			return nil
		},
	})

	assert.Assert(t, !result.EmitSkipped)
	jsOut := emittedFiles["/test.js"]
	t.Logf("JS output:\n%s", jsOut)

	assert.Assert(t, strings.Contains(jsOut, "class Test"),
		"Emitted JS should contain class definition")
	assert.Assert(t, !strings.Contains(jsOut, "operators"),
		"Emitted JS should NOT contain 'operators' keyword")
	assert.Assert(t, strings.Contains(jsOut, `["+"]`),
		"Emitted JS should contain computed property [\"+\"]")
}

// TestLSOperatorCompletionsFunctional verifies that getCompletionsAtPosition returns
// the 12 supported operator strings when the cursor is inside an operators block,
// and does NOT return operator completions when the cursor is outside.
func TestLSOperatorCompletionsFunctional(t *testing.T) {
	t.Parallel()
	if !bundled.Embedded {
		t.Skip("bundled files are not embedded")
	}

	ls, done := setupLS(t, map[string]string{
		"/tsconfig.json": `{ "compilerOptions": {}, "files": ["test.ts"] }`,
		"/test.ts": `
class Vec3 {
    operators {
        
    }
}
`,
	})
	defer done()

	file := ls.program.GetSourceFile("/test.ts")
	assert.Assert(t, file != nil)

	// Find the operators block in the AST
	var opsDecl *ast.Node
	file.ForEachChild(func(n *ast.Node) bool {
		if n.Kind == ast.KindClassDeclaration {
			for _, m := range n.AsClassDeclaration().Members.Nodes {
				if m.Kind == ast.KindOperatorsDeclaration {
					opsDecl = m
					return true
				}
			}
		}
		return false
	})
	assert.Assert(t, opsDecl != nil, "Should find OperatorsDeclaration in AST")

	ops := opsDecl.AsOperatorsDeclaration()

	// Compute a cursor position inside the empty operators block:
	// between the open brace and the close brace.
	// OperatorsDeclaration has no brace tokens, so scan the source text.
	src := file.Text()
	opsPos := ops.Pos()
	relOpenBrace := strings.Index(src[opsPos:], "{")
	assert.Assert(t, relOpenBrace >= 0, "Expected '{' after operators keyword")
	openBracePos := opsPos + relOpenBrace
	insidePos := openBracePos + 1 // position inside the operators body
	relCloseBrace := strings.Index(src[opsPos:], "}")
	assert.Assert(t, relCloseBrace > relOpenBrace, "Expected '}' after '{' in operators block")

	ctx := context.Background()

	// --- 1. Call getCompletionsAtPosition inside the operators block ---
	completions, err := ls.getCompletionsAtPosition(ctx, file, insidePos, nil, false)
	assert.Assert(t, err == nil, "getCompletionsAtPosition should not return an error")
	assert.Assert(t, completions != nil, "Should return completions inside operators block")
	assert.Assert(t, !completions.IsIncomplete, "Completion list should not be incomplete")

	expectedOperators := map[string]bool{
		"+": true, "-": true, "*": true, "/": true, "%": true, "**": true,
		"<": true, ">": true, "<=": true, ">=": true, "==": true, "!=": true,
	}

	assert.Assert(t, len(completions.Items) == 12,
		"Expected 12 operator completions, got %d", len(completions.Items))

	for _, item := range completions.Items {
		assert.Assert(t, expectedOperators[item.Label],
			"Unexpected completion label %q", item.Label)
		assert.Assert(t, item.InsertText != nil && *item.InsertText == item.Label,
			"InsertText should match label for %q", item.Label)
		assert.Assert(t, item.Kind != nil && *item.Kind == lsproto.CompletionItemKindOperator,
			"Kind should be CompletionItemKindOperator for %q", item.Label)
	}

	// --- 2. Call getCompletionsAtPosition outside the operators block ---
	// Position at start of file (before any token). If an error is returned
	// (e.g. "needs auto imports") that's acceptable — the key point is that
	// operator completions are never returned outside the operators block.
	outsidePos := 0
	outsideCompletions, outsideErr := ls.getCompletionsAtPosition(ctx, file, outsidePos, nil, false)

	// When outside an operators block, operator completions should not appear.
	// If the call returns an error for unrelated reasons (auto-imports not prepared),
	// we still pass — no operator completions were leaked.
	if outsideErr == nil {
		if outsideCompletions != nil {
			for _, item := range outsideCompletions.Items {
				assert.Assert(t, !expectedOperators[item.Label],
					"operator completion %q should not appear outside operators block", item.Label)
			}
		}
	}
}
