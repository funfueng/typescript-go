# Operator Overloading in TypeScript Go

**Status:** Implementation complete  
**Last updated:** 2026-06-28

## Overview

Operator overloading allows TypeScript classes to define how built-in operators (`+`, `-`, `*`, etc.) behave for their instances. This is done via an `operators { }` block inside the class body.

This feature is implemented as a **syntax-to-emission transformation** with no runtime library dependency. The operator string IS the emitted method name — there is no mapping layer.

## Syntax

### Declaration

```ts
class Vec3 {
  x: number;
  y: number;

  operators {
    "+"(other: Vec3): Vec3 {
      return new Vec3(this.x + other.x, this.y + other.y);
    }
    "-"(other: Vec3): Vec3 {
      return new Vec3(this.x - other.x, this.y - other.y);
    }
    "-"(): Vec3 {              // no parameters = unary minus
      return new Vec3(-this.x, -this.y);
    }
    "*"(scalar: number): Vec3 {
      return new Vec3(this.x * scalar, this.y * scalar);
    }
  }
}
```

**Key rules:**
- Operator methods are defined inside `operators { }` block
- Operator name is a **string literal** (`"+"`, `"-"`, `"*"`, etc.)
- Binary operators take one parameter (the right operand)
- Unary operators take no parameters (the left operand is `this`)
- Both instance and static operators blocks are supported
- Multiple operators blocks in one class are allowed

### Usage

```ts
declare const a: Vec3;
declare const b: Vec3;

const c = a + b;    // calls a["+"](b)
const d = a - b;    // calls a["-"](b)
const e = a * 2;    // calls a["*"](2)
const f = -a;       // calls a["-"](a)
```

## Complete Operator → String-Key Table

### Binary Operators

Every binary operator in TypeScript maps directly to its string representation:

| Operator | AST Kind | Method Key | Description | Supported |
|---|---|---|---|---|
| `+` | `KindPlusToken` | `"+"` | Addition / concatenation | ✅ |
| `-` | `KindMinusToken` | `"-"` | Subtraction | ✅ |
| `*` | `KindAsteriskToken` | `"*"` | Multiplication | ✅ |
| `/` | `KindSlashToken` | `"/"` | Division | ✅ |
| `%` | `KindPercentToken` | `"%"` | Remainder | ✅ |
| `**` | `KindAsteriskAsteriskToken` | `"**"` | Exponentiation | ✅ |
| `<` | `KindLessThanToken` | `"<"` | Less than | ✅ |
| `>` | `KindGreaterThanToken` | `">"` | Greater than | ✅ |
| `<=` | `KindLessThanEqualsToken` | `"<="` | Less than or equal | ✅ |
| `>=` | `KindGreaterThanEqualsToken` | `">="` | Greater than or equal | ✅ |
| `==` | `KindEqualsEqualsToken` | `"=="` | Equality | ✅ |
| `!=` | `KindExclamationEqualsToken` | `"!="` | Inequality | ✅ |
| `===` | `KindEqualsEqualsEqualsToken` | `"==="` | Strict equality | ✅ |
| `!==` | `KindExclamationEqualsEqualsToken` | `"!=="` | Strict inequality | ✅ |
| `&` | `KindAmpersandToken` | `"&"` | Bitwise AND | ⬜ |
| `\|` | `KindBarToken` | `"\|"` | Bitwise OR | ⬜ |
| `^` | `KindCaretToken` | `"^"` | Bitwise XOR | ⬜ |
| `<<` | `KindLessThanLessThanToken` | `"<<"` | Left shift | ⬜ |
| `>>` | `KindGreaterThanGreaterThanToken` | `">>"` | Right shift | ⬜ |
| `>>>` | `KindGreaterThanGreaterThanGreaterThanToken` | `">>>"` | Unsigned right shift | ⬜ |
| `&&` | `KindAmpersandAmpersandToken` | `"&&"` | Logical AND (short-circuit) | ❌ |
| `\|\|` | `KindBarBarToken` | `"\|\|"` | Logical OR (short-circuit) | ❌ |
| `??` | `KindQuestionQuestionToken` | `"??"` | Nullish coalescing (short-circuit) | ❌ |
| `in` | `KindInKeyword` | `"in"` | Property existence | ❌ |
| `instanceof` | `KindInstanceOfKeyword` | `"instanceof"` | Instance check | ❌ |

### Unary Operators

Unary operators are distinguished from binary by parameter count (0 parameters = unary):

| Operator | AST Kind | Method Key | Context | Supported |
|---|---|---|---|---|
| `-` | `KindMinusToken` | `"-"` | `-a` (prefix unary) | ✅ |
| `+` | `KindPlusToken` | `"+"` | `+a` (prefix unary) | ⬜ |
| `!` | `KindExclamationToken` | `"!"` | `!a` (logical not) | ❌ |
| `~` | `KindTildeToken` | `"~"` | `~a` (bitwise not) | ❌ |

### Assignment Operators

Compound assignments like `+=` are explicitly **not supported**. These are fundamentally different AST nodes (`CompoundAssignment` in native TS, handled via `BinaryExpression` + `OperatorToken` in this codebase). Overloading `+` does NOT automatically make `+=` work.

| Operator | Supported |
|---|---|
| `+=`, `-=`, `*=`, `/=`, `%=`, `**=` | ❌ |
| `&=`, `\|=`, `^=`, `<<=`, `>>=`, `>>>=` | ❌ |
| `&&=`, `\|\|=`, `??=` | ❌ |

### Key
- ✅ Implemented (full pipeline: parse → bind → check → emit)
- ⬜ Not yet verified (mechanism supports it, tests TBD)
- ❌ Out of scope — semantics incompatible with overload model

## Emit Pattern

### Direct Rewrite (1:1, no mapping layer)

The emit transform rewrites operator expressions into element-access method calls:

```
Input                         Output
────────────────────────────────────────────────────
a + b               →        a["+"](b)
a - b               →        a["-"](b)
a * b               →        a["*"](b)
a / b               →        a["/"](b)
a % b               →        a["%"](b)
a ** b              →        a["**"](b)
a < b               →        a["<"](b)
a > b               →        a[">"](b)
a <= b              →        a["<="](b)
a >= b              →        a[">="](b)
a == b              →        a["=="](b)
a != b              →        a["!="](b)
a === b             →        a["==="](b)
a !== b             →        a["!=="](b)
-a                  →        a["-"](a)     // unary: pass self as arg
```

### Design Rationale

There is **no name mapping** — the operator character string IS the emitted property key. This design has several advantages:

1. **Zero runtime dependency** — JavaScript engines see ordinary `a["..."]` property access with no special behavior.
2. **No collision risk** — `"+"` and `"-"` are valid string property keys but are never used as identifiers in normal JS code, so no name conflicts with standard methods like `add()` or `subtract()`.
3. **Predictable output** — The emitted code directly mirrors the source: `a + b` → `a["+"](b)` is trivial to trace and debug.
4. **No mapping table maintenance** — Every PR that adds an operator doesn't need to update a separate operator→method-name registry.

### Chaining and Precedence

Operator precedence is preserved because the transform only changes the **node type**, not the AST structure:

```ts
// Source
a + b * c

// AST before transform
//   BinaryExpression(+)
//   ├── a
//   └── BinaryExpression(*)
//       ├── b
//       └── c

// AST after transform
//   CallExpression
//   ├── ElementAccessExpression["+"]
//   │   └── a
//   └── Arguments
//       └── CallExpression
//           ├── ElementAccessExpression["*"]
//           │   └── b
//           └── Arguments
//               └── c

// Emitted JS: a["+"](b["*"](c))
```

### Emitter Architecture

The transformer hooks into the existing transformer pipeline in `internal/compiler/emitter.go`:

```
getScriptTransformers() pipeline:
  1. MetadataTransformer         (decorator metadata)
  2. TypeEraserTransformer       (strip types)
  3. ImportElisionTransformer    (remove unused imports)
  4. RuntimeSyntaxTransformer    (enum, namespace, param properties)
  5. LegacyDecoratorsTransformer (experimental decorators)
  6. OperatorOverloadTransformer (rewrite binary + unary operator calls)  ✅
  7. JSXTransformer               (JSX → createElement)
  8. ESTransformer                (downlevel ES features)
  9. UseStrictTransformer         (add "use strict")
```

Placement after TypeEraser ensures type information is still available during transformation if needed.

## Architecture

### Compiler Phases

```
Source Text
    │
    ▼
┌──────────────┐
│   Parser     │  parseOperatorsDeclaration(), parseOperatorMethod()
└──────────────┘  Produces: OperatorsDeclaration → OperatorMethodDeclaration[]
    │
    ▼
┌──────────────┐
│   Binder     │  KindOperatorMethodDeclaration → class symbol Members table
└──────────────┘  Symbol name = operator string ("+", "-", etc.)
    │             Symbol flags = SymbolFlagsMethod
    ▼
┌──────────────┐
│   Checker    │  resolveBinaryOperatorOverload() → look up operator key
└──────────────┘  Returns method's return type; suppresses error 2365
    │
    ▼
┌──────────────┐
│   Emitter    │  Transformer rewrites BinaryExpression → ElementAccessExpression
└──────────────┘  a + b → a["+"](b)
    │
    ▼
    JavaScript Output
```

### AST Schema Changes

Two new node kinds added to `_scripts/ast.json`:

- **`OperatorsDeclaration`** — container for operator methods. Extends `ClassElementBase`, appears in class `Members` list.
- **`OperatorMethodDeclaration`** — individual operator definition. Extends `FunctionLikeWithBodyBase`, `ClassElementBase`, `NamedMemberBase`.

Both nodes are generated into `internal/ast/ast_generated.go` with accessors:
- `node.AsOperatorsDeclaration()` / `node.IsOperatorsDeclaration()`
- `node.AsOperatorMethodDeclaration()` / `node.IsOperatorMethodDeclaration()`

`OperatorMethodDeclaration` is also added to the `FunctionLikeDeclaration` type union.

### Key Files

| File | Purpose |
|---|---|
| `_scripts/ast.json` | AST schema definition |
| `internal/ast/ast_generated.go` | Generated AST node types (auto) |
| `internal/parser/parser.go` | Parser: `parseOperatorsDeclaration()`, `parseOperatorMethod()` |
| `internal/binder/binder.go` | Binder: 4 locations added for `KindOperatorMethodDeclaration` |
| `internal/checker/checker.go` | Checker: `resolveBinaryOperatorOverload()`, `resolveUnaryOperatorOverload()`, `resolveOverloadReturnType()` |
| `internal/checker/emitresolver.go` | EmitResolver: `GetOperatorOverload()` — resolves operator overloads at emit time |
| `internal/transformers/tstransforms/operator_overload.go` | Emit transformer: rewrites BinaryExpression + PrefixUnaryExpression → ElementAccessExpression calls |
| `internal/compiler/emitter.go` | Transformer pipeline wiring |

### Checker Flow

When `checkBinaryLikeExpression` encounters a binary operator like `a + b`:

1. Type-checks both operands (`leftType`, `rightType`)
2. Calls `resolveBinaryOperatorOverload(leftType, operator, rightType, errorNode)` **for all operators** (before the per-operator switch)
3. If overload found: returns the method's return type (uses `getReturnTypeOfSignature`; falls back to `any` only when unresolvable)
4. If no overload: falls through to existing arithmetic/string/comparison checks

When `checkPrefixUnaryExpression` encounters `-a`:

1. Type-checks the operand
2. Calls `resolveUnaryOperatorOverload(operandType, operatorNode)` for `-`
3. If overload found: returns the zero-parameter `"-"` method's return type
4. If no overload: falls through to normal unary minus handling

The overload resolution:
1. Unwraps `operandType` via `getApparentType()` to get the instance type
2. Looks up the operator name (e.g., `"+"`) in the type's symbol members via `getMembersOfSymbol()`
3. Falls back to `getPropertyOfType()` as secondary lookup
4. Returns the method's return type from its call signature via `getReturnTypeOfSignature`

## Limitations & Known Gaps

### Resolved (Previously Limitations)

The following were limitations in earlier phases and are now fully implemented:

- **Return type resolution** — `resolveOverloadReturnType` uses `getReturnTypeOfSignature` to resolve actual return types (e.g., `Vec3`, not `any`). Falls back to `any` only when genuinely unresolvable.
- **Unary resolution** — `checkPrefixUnaryExpression` calls `resolveUnaryOperatorOverload` for `-` with zero-parameter lookup.
- **Emit transform** — `OperatorOverloadTransformer` rewrites binary expressions (`a + b` → `a["+"](b)`) and prefix unary expressions (`-a` → `a["-"](a)`). Wired into the `getScriptTransformers()` pipeline.

### Design Limitations (Inherent)

1. **Left-Hand Resolution Only** — Operator lookup is on the left operand's type. `a + b` only checks `typeof a` for an overload, never `typeof b`. This matches Python's `__add__`/`__radd__` model but without the reverse-lookup (`__radd__`). Mixed-type expressions like `number + Vec3` will fall through to standard rules.

2. **No Short-Circuit Operators** — Logical `&&`, `||`, `??` cannot be overloaded because they rely on short-circuit evaluation semantics. Rewriting to a method call would eagerly evaluate both operands, changing semantics.

3. **No Compound Assignment** — `+=`, `-=`, etc. are not supported. These would require a more complex rewrite (e.g., `a += b` → `a = a["+"](b)`) and interact with l-value semantics.

4. **No `instanceof` / `in` Override** — These operators have special runtime semantics in JavaScript that cannot be replicated with method calls.

5. **No Operator on Primitive Types** — Operator overloading only works on object types (classes). Primitive wrappers (`Number`, `String`, `Boolean`) are excluded since they have predefined operator behavior.

6. **No Interface / Type-Level Operators** — Only class instance types support operators. Interfaces with `operators { }` blocks are not supported.

### Out of Scope

7. **Language Service** — No IDE support (hover types, completions in operator context, signature help). These would require new language-service features.

8. **Diagnostic Quality** — No specific error messages for invalid operator names (e.g., using an unsupported operator string like `"&"` or `">>>"` in the `operators` block). Currently these just become regular members and don't participate in overload resolution.

9. **Generic Operators** — No special handling for generic type parameters in operator signatures beyond standard generic resolution.

## Semantics

### Overload Resolution Rules
1. Only types with an explicit `operators { }` block participate in overload resolution
2. Overload lookup is on the **left operand's** type (like Python's `__add__`)
3. If the left type has an operator method, it is used regardless of the right type
4. If no overload is found, normal TypeScript/JavaScript operator rules apply
5. The overload return type must match a call signature — fallback is `any`

### Interaction with Standard Operators
- If a type has `operators { "+"(other: Vec3): Vec3 { ... } }`, then `a + b` resolves via this method
- Every other operator (e.g., `*` if `*` not defined) falls through to standard `number | string` rules
- Mixed overload/no-overload expressions work: `a + b - c` where `+` is overloaded but `-` is not

### Unary Resolution
- Unary `-a` resolves by looking up `"-"` on the type with 0-parameter signature
- If a type defines `"-"(other: X): X` (1 param, binary) and no unary `"-"()`, then `-a` will NOT match — it requires an explicit unary signature
- The unary overload lookup is handled in `checkPrefixUnaryExpression`, distinct from `checkBinaryLikeExpression`

### Missing Return Type Behavior
- If the overload method's signature has no resolved return type (e.g., due to circular references), the checker returns `any` rather than emitting error 2365
- This is a deliberate suppression strategy — better to accept the expression without type-checking than to false-positive reject valid code

## Changelog

| Date | File | Change | Phase |
|---|---|---|---|
| 2026-06-28 | `_scripts/ast.json` | Added `OperatorsDeclaration`, `OperatorMethodDeclaration` kind elements + node definitions | AST Schema |
| 2026-06-28 | `internal/ast/ast_generated.go` | Generated from ast.json | AST Schema |
| 2026-06-28 | `internal/ast/kind_generated.go` | Manual kind additions | AST Schema |
| 2026-06-28 | `internal/ast/kind_stringer_generated.go` | Regenerated | AST Schema |
| 2026-06-28 | `internal/parser/parser.go` | Added `parseOperatorsDeclaration()`, `parseOperatorMethod()` | Parser |
| 2026-06-28 | `internal/parser/parser_operator_test.go` | 3 tests: basic, unary, empty | Parser |
| 2026-06-28 | `internal/parser/backtest_operator_test.go` | Additional parser edge-case tests | Parser |
| 2026-06-28 | `internal/binder/binder.go` | 4 locations: `bindEach`, `declareSymbolAndAddToSymbolTable`, `GetContainerFlags`, container assignment | Binder |
| 2026-06-28 | `internal/binder/binder_operator_test.go` | 2 tests: single operator, multiple operators | Binder |
| 2026-06-28 | `internal/checker/checker.go` | Added `resolveBinaryOperatorOverload()` (L12710), `resolveOverloadReturnType()`, call in `checkBinaryLikeExpression` (L12305) | Checker |
| 2026-06-28 | `internal/checker/checker_operator_test.go` | 3 tests: add no-error, add with-error, subtract | Checker |
| 2026-06-28 | `docs/operator-overloading-skill.md` | Created: phase tracking, feature spec, plans | Docs |
| 2026-06-28 | `docs/operator-overloading.md` | **THIS FILE** — Created: full feature documentation | Docs |
| 2026-06-28 | `internal/checker/checker.go` | Added `resolveUnaryOperatorOverload()` (L12760), call in `checkPrefixUnaryExpression` (L10876) | Checker |
| 2026-06-28 | `internal/checker/checker.go` | Fixed `resolveOverloadReturnType()` to use `getReturnTypeOfSignature` for accurate type resolution | Checker |
| 2026-06-28 | `internal/checker/emitresolver.go` | Added `GetOperatorOverload()` — lazy-resolves both binary and prefix-unary operator overloads | Checker |
| 2026-06-28 | `internal/transformers/tstransforms/operator_overload.go` | Implemented `visitPrefixUnaryExpression` — rewrites `-a` → `a["-"](a)` | Emitter |
| 2026-06-28 | `internal/checker/backtest_operator_test.go` | Edge-case tests: all 12 binary ops, two blocks, void return, inheritance, unary type assertion | Checker |
| 2026-06-28 | `internal/parser/backtest_operator_test.go` | Edge-case tests: all operators, complex body, duplicate name, non-string name | Parser |
| 2026-06-28 | `internal/binder/backtest_operator_test.go` | Edge-case tests: inheritance, multiple classes, static flags, node flags | Binder |
| 2026-06-28 | `internal/compiler/backtest_operator_test.go` | Emit tests: no-crash, multiple operators, comparison, static, chaining, class emitted, unary | Compiler |
| 2026-06-28 | `internal/compiler/impl_operator_test.go` | Emit impl tests: transform, subtract, chaining, plain numbers, class emitted | Compiler |
| 2026-06-28 | `internal/binder/binder_operator_test.go` | Binder tests: single operator, multiple operators | Binder |
| 2026-06-28 | `internal/checker/checker_operator_test.go` | Checker tests: add no-error, add with-error, subtract | Checker |
| 2026-06-28 | `internal/checker/impl_operator_test.go` | Checker impl tests: multiply, chain add, unary minus, static operators | Checker |
