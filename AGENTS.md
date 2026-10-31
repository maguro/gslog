# AGENTS.md

## Required reads

Before making any code or test edits, read `CODING_STANDARDS.md`. It holds the
rules that apply to every edit. If this file and `CODING_STANDARDS.md` differ,
follow the stricter rule.

## Skills

Use these skills from `.claude/skills/` when the task matches:

| Skill | Use it when |
|---|---|
| `go-testing` | Writing or changing a test |
| `go-benchmarks` | Changing a hot path listed below |
| `git-commit-message` | Writing a commit message |
| `asd-ste100` | Writing or checking a code comment or a commit body |

## Project overview

- Language: Go
- Module: `m4o.io/gslog`, a published library
- What the code does: see `README.md` and the package doc comments
- Go version: see `go.mod`. CI builds every version in the matrix in
  `.github/workflows/ci.yml`.

## Commands

- Format: `make fmt`
- Format check: `make check-fmt`
- Unit tests, with the race detector: `make test`
- Coverage: `make coverage`
- Lint: `make lint`
- Vulnerability scan: `make vuln`
- Regenerate `docs/examples.md`: `make docs`
- Check `docs/examples.md`: `make check-docs`
- Tidy modules: `make tidy`
- Gate target: `make check`
- CI target, the gate and coverage: `make ci`

`make check` runs `test`, `lint`, `check-fmt`, and `check-docs`.

## Hot paths

The `go-benchmarks` skill applies to the log path: `Handle`, `WithAttrs`,
`WithGroup`, and any package they call for each record.

The benchmark command:

```sh
go test ./gcp/benchmark/ ./stdout/benchmark/ -run '^$' -bench . -benchmem
```

This repo has no compare command, so alternate the base and the working tree by
hand, as the skill describes.
