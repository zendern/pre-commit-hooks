# pre-commit-hooks

[![ci](https://github.com/zendern/pre-commit-hooks/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/zendern/pre-commit-hooks/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go_1.26-00ADD8?logo=go&logoColor=white)](https://go.dev) [![pre-commit](https://img.shields.io/badge/pre--commit-hooks-FAB040?logo=precommit&logoColor=white)](https://pre-commit.com) [![Task](https://img.shields.io/badge/Task-29BEB0?logo=task&logoColor=white)](https://taskfile.dev) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Shared [pre-commit](https://pre-commit.com) hooks for zendern's repositories.
They use the standard `.pre-commit-hooks.yaml` format, so they run under
pre-commit or [prek](https://prek.j178.dev) alike. Public, so CI (and public
repos) can fetch them without credentials.

## Usage

In `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/zendern/pre-commit-hooks
    rev: <full commit sha>  # frozen: v1.0.0
    hooks:
      - id: no-latest-runners
```

Freeze `rev` to a commit SHA (`prek autoupdate --freeze` or
`pre-commit autoupdate --freeze`); Renovate's pre-commit manager bumps it.

The hooks are Go programs (`language: golang`). pre-commit and prek build them
with `go install` on first use, and download a Go toolchain if none is
installed.

## Hooks

### `no-latest-runners`

Fails when a workflow job runs on a floating runner label such as
`ubuntu-latest`, `macos-latest` or `windows-latest`. GitHub repoints those
labels to new images (`ubuntu-latest` moved to Ubuntu 26 from 2026-10-19), so
an unpinned job changes OS without a commit. Pin a version instead:
`ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, ...

It parses each workflow and reads `jobs.<id>.runs-on` in every form: a label, a
list of labels, or a `group`/`labels` object. Labels set through an expression
(`runs-on: ${{ matrix.os }}`) can't be resolved and are skipped.

```
.github/workflows/ci.yml:12: job "build" runs on "ubuntu-latest"
```

#### Exclusions

- One job: put `# no-latest-runners: allow` on its `runs-on` line (or on a
  list item's line).

  ```yaml
  runs-on: ubuntu-latest # no-latest-runners: allow
  ```

- Whole files: add `exclude:` to the hook in your `.pre-commit-config.yaml`.

  ```yaml
      hooks:
        - id: no-latest-runners
          exclude: ^\.github/workflows/experimental\.yml$
  ```

## Development

With [Task](https://taskfile.dev):

```sh
task check   # gofmt, vet, tests, and the hooks against this repo's workflows (what CI runs)
task test    # tests only
```
