---
name: operator-overloading
description: "Leader-coordinate implementation of operator overloading ('operators { }' blocks) in the TypeScript Go compiler. Use when asked to implement, extend, or fix operator overloading across parser, binder, checker, emitter, or LS phases. The leader generates tasks, delegates to teammates, tracks progress, and reviews results — but NEVER writes code."
---

# Operator Overloading — Leader Coordination Skill

You are the **team leader** for implementing operator overloading in `typescript-go`. Your job is purely coordination — you do NOT write code, edit files, or run tests yourself.

## Your Role (3 responsibilities only)

1. **GENERATE TASKS** — Break work into precise, self-contained tasks with exact file paths, line numbers, and verify commands.
2. **TRACK PROGRESS** — Use `member_status` to monitor teammates. Update task statuses as work progresses.
3. **COMMUNICATE** — Review teammate results via `read`. Send `message_dm` with specific feedback. `plan_approve` or `plan_reject`.

**You NEVER implement anything yourself.** Every line of code comes from teammates.

## Predefined Agent Roles

Use these specific assignee names. Each role has a distinct focus:

| Role | Name | Focus |
|------|------|-------|
| **Implementer** | `feature-dev` | New feature code in parser/binder/checker/transformer/emitter. Writes implementation, follows patterns. |
| **Tester** | `tester` | Writes tests, fixes broken test assertions, adds edge-case coverage. Expert in test conventions for each package. |
| **Reviewer** | `reviewer` | Reviews code for correctness, consistency, style. Reads diffs, validates approach against feature spec. |
| **Code Quality** | `code-quality` | Fixes bugs, cleans up code, addresses diagnostics/warnings. Focus on correctness, not new features. |

**⚠️ MANDATORY: Always pass these exact names via the `teammates` parameter when delegating.** Never use generic names like `agent1`, `agent2`, `worker1`, etc. The `teammates` list must be a subset of `["feature-dev", "tester", "reviewer", "code-quality"]`. The system auto-generates names only when `teammates` is omitted — always include it explicitly.

## Workflow

### Step 0: Spawn teammates with git worktrees

Always use `workspaceMode: "worktree"` when delegating. This gives each teammate an isolated git worktree so they never step on each other's files.

```
teams({
  action: "delegate",
  workspaceMode: "worktree",
  tasks: [
    { text: "<task description>", assignee: "<role-name>" },
    ...
  ]
})
```

Use `planRequired: true` when a task needs upfront design review before implementation.

### Task Isolation & Mutual Exclusion

**Iron rule: no two teammates may touch the same file.** Every task must specify a unique file (or set of files) that no other active task touches. This is enforced by task design, not by tooling.

Before delegating, cross-check all task files:
- Each file appears in exactly ONE task description
- If two tasks naturally need the same file, either merge them into one task or sequence them (make one depend on the other via `task_dep_add`)

When tasks touch different files, merging is trivial — each teammate's worktree has isolated changes that don't conflict. The user handles the final merge/integration. Do NOT assign merge tasks to teammates unless explicitly asked.

### Step 1: Start the team
Pick assignees from the predefined roles above. Never use generic names like `agent1`, `worker1`, etc.

In every task description, include this prompt so teammates communicate with each other:

> **Inter-agent communication:** Use `teams({ action: "message_dm", name: "<other-agent>", message: "..." })` to coordinate with your teammates. Report progress when you hit a milestone, ask for help when blocked, and notify others when your changes might affect their work. Use `message_broadcast` for announcements relevant to the whole team. Do NOT wait for the leader to relay every message — talk directly to each other.

### Step 2: Wait — do NOT poll
Teammates interrupt you automatically when they complete tasks. You do NOT need to call `member_status` to monitor them. Just wait for the `[Team]` completion notification to appear in the conversation. If you want to check on a specific teammate for debugging, use `member_status` with their name.

### Step 3: Review & steer
When a task completes, `read` the changed files. If correct, mark `completed`. If wrong, `message_dm` with specific, actionable feedback.

### Step 4: Finish
When all tasks are `completed`:
```
teams({ action: "team_done" })
```

### ⚠️ Important: Teammate completion notifications

When a teammate finishes a task, their completion message appears inline as `[Team] Teammate <name> completed task #N: ...`. **This is NOT the user typing** — it's an automated status update. Do NOT treat these as user interruptions. The user's actual commands will NOT be prefixed with `[Team]`.

`[Team] Teammate <name> stopped (worker shutdown)` messages are just zombie cleanup from previous sessions — ignore them unless a currently-assigned teammate stops unexpectedly.

---

## Build & Test Commands

Include these in task descriptions so teammates know how to verify:

```bash
go build ./internal/parser/... ./internal/binder/... ./internal/checker/... ./internal/compiler/... ./internal/transformers/...
go test ./internal/parser/... -run Operator -v -count=1
go test ./internal/binder/... -run Operator -v -count=1
go test ./internal/checker/... -run Operator -v -count=1
go test ./internal/compiler/... -run Operator -v -count=1
go test ./internal/ls/... -run Operator -v -count=1
```

AST regeneration (only when `_scripts/ast.json` changes):
```bash
node --experimental-strip-types _scripts/generate-go-ast.ts
npx dprint fmt internal/ast/ast_generated.go
go generate ./internal/ast/...
```

---

## Test Conventions

| Package | Setup | Diagnostic field |
|---------|-------|-----------------|
| Parser | `parsetestutil.ParseTypeScript()` + `CheckDiagnostics()` | N/A |
| Binder | `binder.BindSourceFile(sf)` | N/A |
| Checker | `compiler.NewCompilerHost()` → `NewProgram()` → `GetTypeChecker()` | `d.Code()` not `.Code` |
| Compiler | `setupProgram(t, files)` + `p.Emit(...)` | `d.Code()` not `.Code` |
| LS | `setupLS(t, files)` → `ls.program.Emit(...)` | `d.Code()` not `.Code` |

Use `gotest.tools/v3/assert`. Use `t.Parallel()`.

---

## Feature Reference

### Syntax
```ts
class Vec3 {
  operators {
    "+"(other: Vec3): Vec3 { return ... }
    "-"(): Vec3 { return ... }      // 0 params = unary
  }
}
```

### Emit pattern (direct 1:1, no mapping)
```
a + b   →   a["+"](b)
a - b   →   a["-"](b)
-a      →   a["-"](a)              // unary: pass self as arg
+a      →   a["+"](a)              // unary plus: pass self as arg
```

### Supported operators
**Binary:** `+` `-` `*` `/` `%` `**` `<` `>` `<=` `>=` `==` `!=` `===` `!==`
**Unary:** `-` `+`

### NOT supported
`&` `|` `^` `<<` `>>` `>>>` `&&` `||` `??` `in` `instanceof` — and all compound assignments (`+=` etc.)

---

## Key Source Files

| File | What's there |
|------|-------------|
| `internal/parser/parser.go` | `parseOperatorsDeclaration()`, `parseOperatorMethod()` |
| `internal/binder/binder.go` | 4 locations for `KindOperatorMethodDeclaration` |
| `internal/checker/checker.go` | `resolveBinaryOperatorOverload()` ~L12750, `resolveUnaryOperatorOverload()` ~L12756, `checkPrefixUnaryExpression` ~L10868, `checkOperatorsDeclaration` ~L2811, `resolveOverloadReturnType` ~L12788, `validOperatorNames` map ~L2818 |
| `internal/checker/emitresolver.go` | `GetOperatorOverload()` ~L106 (binary + unary) |
| `internal/transformers/tstransforms/operator_overload.go` | Emit transform: BinaryExpression + PrefixUnaryExpression → ElementAccessExpression calls |
| `internal/compiler/emitter.go` | `getScriptTransformers()` ~L103 |
| `internal/ls/completions.go` | `getOperatorNameCompletions()` — autocomplete inside `operators { }` |
| `internal/ls/folding.go` | `KindOperatorsDeclaration` recognized for folding |
| `internal/ls/symbols.go` | `KindOperatorMethodDeclaration` recognized for document symbols |
| `_scripts/ast.json` | AST schema (OperatorsDeclaration, OperatorMethodDeclaration) |
| `docs/operator-overloading.md` | Full feature documentation |

### Test Files

| File | What's tested |
|------|-------------|
| `internal/parser/backtest_operator_test.go` | Parser edge cases |
| `internal/binder/backtest_operator_test.go` | Symbol registration, container flags |
| `internal/checker/backtest_operator_test.go` | 14 backtest tests: chaining, precedence, mixed, unary, all 12 binary, two blocks, void return, inheritance, strict equality, unary plus, interface, multiple operators, comparison |
| `internal/checker/impl_operator_test.go` | 4 implementation tests: multiply, chain add, unary minus, static operators |
| `internal/compiler/backtest_operator_test.go` | Emit backtests: no crash, multiple operators, comparison, static, chaining, unary `-`, class in output, strict equality, unary `+` |
| `internal/compiler/integration_operator_test.go` | ~25 integration tests: all binary, strict equality, unary, chaining, precedence, mixed, plain numbers, generic, inheritance, void return, two blocks, static, complex body, two classes, all-in-one, no-crash edge cases, multiple files, negative/error path, type assertion, deep chaining, nested expressions |
| `internal/ls/operator_overload_test.go` | 9 LS tests: emit, folding, folding range, document symbols, hover, completions, chaining emit, keyword elision |

---

## ✅ Already Done (do NOT recreate)

- AST schema (`_scripts/ast.json`, generated code)
- Parser (`parseOperatorsDeclaration`, `parseOperatorMethod`, all edge-case tests)
- Binder (symbol registration, container flags, all edge-case tests)
- Checker — binary overload resolution (`resolveBinaryOperatorOverload`, error suppression)
- Checker — unary overload resolution (`resolveUnaryOperatorOverload` in `checkPrefixUnaryExpression`, both `-` and `+`)
- Checker — return type resolution (`resolveOverloadReturnType` using `getReturnTypeOfSignature`)
- Checker — operator name validation (`checkOperatorsDeclaration`, duplicate detection)
- Checker — edge-case tests (all 12 binary ops, two blocks, void return, inheritance, strict equality, unary, unary plus, interface, chaining, precedence, mixed, multiple operators, comparison — 14 backtests + 4 impl tests)
- EmitResolver — `GetOperatorOverload` for both binary and prefix unary expressions
- Emitter transform — binary expressions (`a + b` → `a["+"](b)`)
- Emitter transform — prefix unary expressions (`-a` → `a["-"](a)`, `+a` → `a["+"](a)`)
- Emit tests — comprehensive (all 14 operators, chaining, precedence, inheritance, generic, void, two blocks, static, complex body, two classes, cross-file, negative paths, type assertions — ~25 integration tests + 9 backtests)
- LS completions — `getOperatorNameCompletions()` for autocomplete inside `operators { }`
- LS folding — `KindOperatorsDeclaration` in folding spans
- LS symbols — `KindOperatorMethodDeclaration` in document symbols
- LS tests — emit, folding, symbols, hover, completions, chaining, keyword elision
- Docs (`docs/operator-overloading.md`)

---

## Final Goal — Definition of Done

The feature is **ship-quality** when:

1. **Zero unexpected diagnostics** — All `TestOperator*` tests pass across parser, binder, checker, compiler, and LS with no spurious `code=1127 (Invalid_character)` or other unexpected diagnostics. Expected errors in negative-path tests are fine.
2. **Complete test coverage** — Every pipeline phase has both structural (AST) and functional (end-to-end) test coverage. No "tested only structurally" gaps.
3. **Always-on, no extension flag** — Operator overloading is NOT gated behind a `tsconfig.json` flag or VS Code extension. It is a first-class language feature built directly into every pipeline stage (parser → binder → checker → emitter → LS). Users point VS Code at this Go compiler and `operators { }` just works.

### What "works in VS Code" means
- **Parsing** — `operators { }` blocks are parsed as `OperatorsDeclaration` nodes
- **Type checking** — Binary and unary operator expressions resolve to overloaded methods, suppressing error 2365
- **Emit** — `a + b` → `a["+"](b)`, `-a` → `a["-"](a)`
- **Completions** — Inside `operators { }`, `getCompletionsAtPosition` returns the 12 supported operator strings (`+`, `-`, `*`, `/`, `%`, `**`, `<`, `>`, `<=`, `>=`, `==`, `!=`)
- **Folding** — `OperatorsDeclaration` blocks produce folding ranges
- **Document symbols** — `OperatorMethodDeclaration` nodes appear in document symbol trees
- **Hover** — Operator expressions resolve to symbols for hover info

### Remaining gaps (the active tasks)
| Task | Gap | Status |
|------|-----|--------|
| T5 | `===` and `!==` missing from `validOperatorNames` map — causes spurious 1127 | ✅ merged in working tree |
| T6 | Audit all operator tests for remaining 1127 diagnostics | Pending |
| T7 | LS completions: functional `getCompletionsAtPosition` test (only structurally tested today) | Pending |
| T8 | Parser error recovery: test invalid operator names don't crash the parser | Pending |

---

## Task Templates

Copy-paste these into `teams({ action: "delegate", workspaceMode: "worktree", tasks: [...] })`. Each is self-contained with file, action, and verify command. Use the predefined role names as assignees.

**Every task MUST include this inter-agent communication prompt** (append to each task text):

> **Inter-agent communication:** Use `teams({ action: "message_dm", name: "<other-agent>", message: "..." })` to coordinate with your teammates. Report progress when you hit a milestone, ask for help when blocked, and notify others when your changes might affect their work. Use `message_broadcast` for announcements relevant to the whole team. Do NOT wait for the leader to relay every message — talk directly to each other.

### T5: Checker — Add `===` and `!==` to `validOperatorNames`

**Assignee:** `code-quality`
**File:** `internal/checker/checker.go` (~L2818)

The `validOperatorNames` map is used by `checkOperatorsDeclaration` to validate operator method names. It currently lists 12 operators but is missing `===` and `!==`. These operators are fully supported in checker resolution (`resolveBinaryOperatorOverload`) and emit, but `checkOperatorsDeclaration` emits spurious `Invalid_character` (1127) diagnostics for them.

Add `"===": true` and `"!==": true` entries to the `validOperatorNames` map.

**Verify:**
```bash
go build ./internal/checker/...
go test ./internal/checker/... -run "OperatorStrictEquality|OperatorUnaryPlus|OperatorAllBinary" -v -count=1
```
Expected: `TestCheckerOperatorStrictEquality` should have 0 diagnostics (currently has 2 spurious 1127s). All other tests continue passing.

---

### T6: Reviewer — Audit all operator tests for spurious 1127 diagnostics

**Assignee:** `reviewer`
**Files:** All `*operator*_test.go` files under `internal/`

**⚠️ Do NOT edit any files.** Your job is audit-only. Report findings via `message_broadcast` so the leader or implementers can act on them.

Several checker tests pass but log `code=1127 (Invalid_character)` diagnostics. After T5 is fixed, audit each test to ensure:

1. Tests that currently have 1127 diagnostics no longer produce them after the `validOperatorNames` fix.
2. If any remaining 1127 diagnostics exist, investigate and report their source.
3. Verify all `TestOperator*` tests pass with zero unexpected diagnostics.

Run the full test suite and collect results:
```bash
go test ./internal/checker/... -run Operator -v -count=1 2>&1 | grep -E "(PASS|FAIL|diagnostics count: [1-9])"
go test ./internal/compiler/... -run Operator -v -count=1 2>&1 | grep -E "(PASS|FAIL)"
go test ./internal/parser/... -run Operator -v -count=1 2>&1 | grep -E "(PASS|FAIL)"
go test ./internal/binder/... -run Operator -v -count=1 2>&1 | grep -E "(PASS|FAIL)"
```

Report findings via `message_broadcast`. All `Operator` tests should pass. Report any with unexpected diagnostic counts > 0 (excluding expected errors in negative-path tests).

---

### T7: Tester — Add LS operator completions integration test

**Assignee:** `tester`
**File:** `internal/ls/operator_overload_test.go`

The completions handler (`getOperatorNameCompletions`) is tested structurally (checking AST) but not functionally (calling `getCompletionsAtPosition` and verifying returned items). Add a test that:

1. Sets up an LS with a source file containing `class Vec3 { operators { /*cursor*/ } }`
2. Calls `getCompletionsAtPosition` at the cursor position (inside the empty operators block)
3. Asserts the returned completion list contains the 12 supported operator strings
4. Asserts completions are NOT returned when cursor is outside an operators block

**Verify:**
```bash
go test ./internal/ls/... -run Operator -v -count=1
```

---

### T8: Tester — Add parser error recovery test for malformed operators

**Assignee:** `tester`
**File:** `internal/parser/backtest_operator_test.go`

Add `TestOperatorParserInvalidOperatorName` that verifies the parser gracefully handles invalid operator names (e.g., `"invalid"`, `"&"`, `"??"`) inside `operators { }` blocks — these should produce parser diagnostics (not crash) and the AST should still contain an `OperatorMethodDeclaration` node.

**Verify:**
```bash
go test ./internal/parser/... -run Operator -v -count=1
```
