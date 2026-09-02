# AGENTS.md

## Scope

This file defines code organization, Go style, documentation, testing, and quality rules only.

Functional library requirements belong to `tz.md`. DSL grammar and examples belong to `DSL v1.1.md`. Do not duplicate or reinterpret either specification in this file.

## Go baseline

- Use module path `dslparser` and Go `1.26`.
- Use only the Go standard library.
- Format every Go file with `gofmt`.
- Write idiomatic Go; prefer clear standard patterns over custom frameworks.
- Do not use `unsafe`, reflection, `map[string]any`, or other untyped containers for structured data.
- Do not add mutable global state.
- Do not add external dependencies, vendored code, or generated third-party code.

## Package organization

- Keep the public package at the module root.
- Keep implementation details under `internal/`.
- Group code by stable responsibility, not by individual DSL tag.
- Keep every package focused on one clear purpose.
- Maintain a directed dependency graph and prevent cyclic imports.
- Do not let an internal package import the root package.
- Do not create forwarding packages, parallel implementations, or wrappers without a real boundary.

## API and type design

- Keep the exported API minimal and limited to requirements in `tz.md`.
- Keep fields unexported unless direct mutation is an intentional public contract.
- Prefer concrete types. Introduce an interface only when a real substitution boundary or multiple implementations exist.
- Prefer named function types and registration tables for dispatch.
- Use ordinary `int`, `string`, `bool`, and slices unless a distinct type enforces a stable contract.
- Preserve ordered data with slices, not map iteration.
- Make ownership explicit. Copy slices or other mutable aggregates before exposing internal state.
- Do not expose hidden mutation through pointers, caches, or accessors.
- Avoid speculative abstractions intended only for possible future versions.

## Functions and control flow

- Give each function one concrete responsibility.
- Prefer small functions with explicit inputs and results.
- Treat approximately 30 logical lines as a signal to check for mixed responsibilities.
- Split a function only when the extracted part has a meaningful name and independently testable contract.
- Do not split linear code into trivial one-line wrappers merely to reduce line count.
- Prefer early returns and shallow control flow.

## Naming and readability

- Follow standard Go naming conventions and use concise, descriptive identifiers.
- Use established initialisms and short receiver names consistently.
- Avoid package-name repetition in exported identifiers.
- Avoid vague names when a more specific role is available.
- Group constants by concept and prefer readable code over clever expressions.

## Errors and state

- Return errors idiomatically.
- Add useful context with `fmt.Errorf` and `%w` when preserving an underlying error.
- Do not compare errors by message text when a stable type or sentinel is appropriate.
- Do not use `panic` for recoverable input or control flow.
- Do not catch `panic` as normal error handling.
- Keep state local to a single operation unless immutable shared state is sufficient.
- Do not reuse mutable state across independent operations.

## Documentation comments

- Write all Go documentation comments in Russian.
- Document every package, exported and unexported type, function, method, interface, constant or constant group, package-level variable, and non-trivial struct field.
- Begin every identifier comment with the exact identifier name, including unexported identifiers.
- State the purpose first; add non-obvious contracts, units, side effects, and error behavior.
- Explain both `true` and `false` for non-obvious Boolean results.
- State explicitly when a range has an exclusive end.
- Do not narrate implementation line by line or use placeholder comments.
- Do not leave `TODO` markers for requirements that belong to the current work.
- Give every package one package comment, preferably in `doc.go`.

## Testing

- Write tests before production implementation.
- Prefer table-driven tests for sets of related cases.
- Name tests after observable behavior, not internal implementation steps.
- Keep tests deterministic and independent of execution order.
- Use `t.Helper()` for test helpers and `t.Run()` for meaningful case names.
- Use `t.Parallel()` only when shared mutable state and ordering cannot affect the test.
- Store substantial fixtures under `testdata/`.
- Do not weaken, delete, or skip a valid test to make an implementation pass.
- Use only `testing` and Go's built-in fuzzing support.
- Preserve every reproduced fuzz defect as a deterministic regression test or seed.

## Verification

Before final completion, all commands below must succeed from the module root:

```text
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go list ./...
go doc dslparser
```

Also verify that the module has no external dependencies, cyclic imports, data races, stale documentation comments, or unintended exported identifiers.
