# Project conventions

These apply to anyone — human or AI, on any model — working in this repo. They
are conventions about *how* to work here, not a description of what the code
does; read `README.md` and the package doc comments for that.

## Go style baseline

This repo's Go style baseline is the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).
Follow it for anything this file doesn't cover — method and function ordering,
error handling and naming, receiver types, struct literals, and so on. Where a
rule below conflicts with Uber's guide, this file wins; treat everything below
as this project's stated exceptions and additions, not a full restatement.

`.golangci.yml` enforces the subset of the guide a linter can check
mechanically (`unparam`, `unconvert`, `ineffassign`, `gosec`, `funcorder`,
`gocritic`'s `opinionated`/`style` tags, and similar). The rest — naming and
grouping — isn't linter-checkable and depends on the `/code-review` pass
below.

This module targets the Go version declared in `go.mod`, and CI builds every
version in the matrix in `.github/workflows/ci.yml`. Do not use language or
standard-library features newer than the declared minimum.

## Before declaring work complete

For any change of nontrivial size, run a review pass against this file's
conventions before calling it done — comment content, test framework choice,
formatting, adherence to the Go style baseline above, everything above and
below this section. Use `/code-review` against the diff rather than relying
on writing-time self-checking alone; a dedicated review pass catches drift a
single pass of authoring misses.

A change that touches code is not proposed as done until `make check` passes,
run in the actual working tree, not a scratch copy or worktree.

## Code comments

All code comments must follow ASD-STE100 (Simplified Technical English):
short sentences, one instruction per sentence, active voice, approved
vocabulary, no jargon or idioms. Use the `asd-ste100` skill to write or check
comments against this standard — do not rely on general-purpose simplifying.

## Code comments state facts, never rationale or future work

A code comment documents *what* the code does and *how to use it* — purpose,
parameters, return values, usage notes, or a bare fact/invariant — and must
never restate *why* a design decision was made, or note a provisional or
future state.

**Never restate rationale.** No "because X", "so that Y", and no em-dash
justification. A design decision that needs an argument belongs in the commit
message or in the pull request, not in the source.

**Never note future or provisional state.** No "not implemented yet", "will be
cleaned up later" — this warns no one outside the repo and just rots. If a
stub needs a note, put it in the function body next to the `panic`/stub return
so it's deleted with the code it describes.

**Self-check before calling a file done:** for every comment you wrote or
touched, apply the audience test — caller concerns (behavior, contract,
consequences of use) belong in the doc comment; maintainer concerns (why
obvious-looking code is wrong) belong at the definition site as a bare fact,
not an essay. That covers invariants the compiler can't catch (an implicit
lock order, a slice that must stay sorted), not exported signatures, where
the diff itself is the warning.

## Where the package doc comment lives

Every non-`main` package has a package comment, in exactly one file, starting
with the package name: `// Package gslog ...`.

A package with more than one source file puts that comment in `doc.go`. That
file holds the package comment and nothing else, except `//go:generate`
directives. A package with exactly one source file puts the comment at the top
of that file; the filename does not matter, and a file named after the package
does not exempt a multi-file package from `doc.go`.

## Testing

**Match the framework to the test.** Ginkgo (`ginkgo/v2`) for complicated
behavioral tests — branching setup, a fixture shared across many related cases,
or timing and concurrency to pin down; its containers and Gomega's
`Eventually`/`Consistently` earn their weight there. Plain `testing.T` for
simple features: a handful of independent assertions, a table of
input-to-output cases, a single fact. Reaching for Ginkgo to assert one thing
costs more than it returns. Do not convert an existing test to match — convert
only when you are already changing it for another reason.

**Assertions follow the framework:** Gomega (`Expect`) inside Ginkgo specs,
`testify` (`assert`/`require`) in plain `testing.T` tests, where it clarifies
intent. Do not mix them in one test — `require` inside a spec bypasses
Ginkgo's failure reporting.

**Examples are tests.** `example_test.go` runs under `go test` and its
`// Output:` blocks are assertions. Update them with the behavior they check.

## Code formatting beyond gofmt

gofmt does not enforce these; apply them by hand, in production and test code
alike.

- **Method ordering.** This intentionally deviates from Uber's guide, which
  sorts by call order and allows an unexported helper to sit next to its one
  caller; this project instead partitions every type's methods into exported
  before unexported, favoring breadth-first reading of a type's public
  surface over call-order locality. Group methods by receiver, with a
  `NewXYZ` constructor immediately after its type definition. Place every
  exported method before any unexported one, so a reader can take in the
  type's public surface breadth-first without an unexported implementation
  detail interrupting it; order the exported methods themselves in rough
  call order (e.g., `Enabled` before `Handle`). If the type implements more
  than one interface, group its exported methods by the interface they
  satisfy rather than interleaving them. Unexported helpers follow, each
  ordered near the exported method it supports. Unattached utility functions
  go last in the file.
- **Single-line bodies.** A method/function body shares the signature's line only
  when it is empty (`{}`) or a single `return` of a bare value — a field,
  identifier, or literal with no call. Anything that does work — a `return`/`panic`
  whose expression calls something, multiple statements, or control flow — goes on
  its own line.
- **Blank-line grouping.** Separate a function's logical steps with a single blank
  line so its structure is visible at a glance — for example, set a guard clause
  off from the work it protects, or the result-producing step off from what builds
  it. Conversely, keep tightly-coupled statements together (e.g. a loop's
  accumulator seed stays with its loop).
- **No calls inside call arguments.** Hoist a function or method call out of
  another call's argument list into a named local. Two reasons, and either alone
  is enough: stepping into the outer call in a debugger otherwise walks the inner
  calls first, landing somewhere other than the function under inspection; and
  the name given to the result documents what the call returns, which the call
  itself often leaves unclear. Skip the hoist only where neither applies — the
  outer call is one you never step into, such as a stdlib helper
  (`filepath.Base(fset.Position(f.Pos()).Filename)`) or an assertion wrapper
  (`Expect(...)`, `require.NoError(...)`); or the inner call is a trivial
  accessor whose own name already says what it returns. Both stay as they are.

## The public API is a contract

This module is a published library. Every exported symbol outside `internal/`
is part of its API. Do not rename, remove, or change the signature of an
exported symbol as a side effect of other work. A breaking change needs its
own change, and a note in the pull request that describes it.
