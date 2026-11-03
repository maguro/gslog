# Contributing to gslog

## Get gslog

Add gslog to a Go module with `go get m4o.io/gslog`. The
[README](README.md#install) gives the full setup steps.

## Report a bug

Report a bug in a GitHub issue:

https://github.com/maguro/gslog/issues/new

Include these items in the report:

- The version of gslog and the version of Go.
- The handler that you use, `gcp` or `stdout`, and where the program runs.
- The steps that show the problem.
- The result that you expected and the result that you got.

Do not report a security vulnerability in a public issue. Use the process in
[SECURITY.md](SECURITY.md).

## Ask for a feature

Open a GitHub issue that describes the problem to solve. Open the issue
before you write the code. A change to the public API or to the fields of a
log entry needs agreement before work starts.

## Send a change

1. Fork the repository and make a branch from `main`.
2. Make the change.
3. Run `make check`. This command runs the tests with the race detector, the
   linter, the format check, and the check of `docs/examples.md`.
4. Open a pull request against `main`. Refer to the related issue in the
   description.

The maintainer reviews each pull request. CI must pass before a merge.

## Requirements for a change

A change must obey the project conventions in
[CODING_STANDARDS.md](CODING_STANDARDS.md) and [AGENTS.md](AGENTS.md). These
conventions apply to all contributors. Their baseline is the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).

- Add tests for new behavior. Add a test that shows the bug for a bug fix.
- Keep `make check` passing.
- Write code comments in ASD-STE100 Simplified Technical English.
- Run `make docs` after you change an example, so that `docs/examples.md`
  shows the change.
- Measure a change to the log path, and report the result in the commit
  message. The [`go-benchmarks` skill](.claude/skills/go-benchmarks/SKILL.md)
  gives the procedure, and [AGENTS.md](AGENTS.md#hot-paths) lists the paths.
- Write commit messages that obey the
  [`git-commit-message` skill](.claude/skills/git-commit-message/SKILL.md).

## License

gslog uses the [Apache License 2.0](LICENSE). When you send a contribution,
you release it under this license.
