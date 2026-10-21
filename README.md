# pre-commit-hooks

[![ci](https://github.com/zendern/pre-commit-hooks/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/zendern/pre-commit-hooks/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go_1.26-00ADD8?logo=go&logoColor=white)](https://go.dev) [![pre-commit](https://img.shields.io/badge/pre--commit-hooks-FAB040?logo=precommit&logoColor=white)](https://pre-commit.com) [![Task](https://img.shields.io/badge/Task-29BEB0?logo=task&logoColor=white)](https://taskfile.dev) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)


Shared [pre-commit](https://pre-commit.com) hooks.


They use the standard `.pre-commit-hooks.yaml` format, so they run under
pre-commit or [prek](https://prek.j178.dev) alike.

## Contents

- [Usage](#usage)
- [Hooks](#hooks)
  - [GitHub Actions](#github-actions)
    - [`no-latest-runners`](#no-latest-runners): pin runner labels instead of `*-latest`
    - [`require-timeouts`](#require-timeouts): require `timeout-minutes` on jobs (and steps)
- [Development](#development)
  - [Releases](#releases)

## Usage

In `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/zendern/pre-commit-hooks
    rev: <full commit sha>  # frozen: v1.1.0
    hooks:
      - id: no-latest-runners
      - id: require-timeouts
```

> [!NOTE]
> Suggestion:  Freeze `rev` to a commit SHA (`prek autoupdate --freeze` or
> `pre-commit autoupdate --freeze`); You can also use Renovate's pre-commit manager to bump it.

The hooks are Go programs (`language: golang`). pre-commit and prek build them
with `go install` on first use, and download a Go toolchain if none is
installed.

## Hooks

### GitHub Actions

These hooks check workflow files in `.github/workflows/`.

#### `no-latest-runners`

Fails when a workflow job runs on a floating runner label such as
`ubuntu-latest`, `macos-latest` or `windows-latest`. GitHub repoints those
labels to new images (ex. `ubuntu-latest` moved to Ubuntu 26 starting on 2026-10-19), so
an unpinned job changes OS without a commit potentially breaking builds since underlying tooling changes.

Pin a version instead:
`ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, ...

It parses each workflow and reads `jobs.<id>.runs-on` in every form: a label, a
list of labels, or a `group`/`labels` object.

> [!WARNING]
> Labels set through an expression
> (`runs-on: ${{ matrix.os }}`) can't be resolved and are skipped.

Failure output

```
.github/workflows/ci.yml:12: job "build" runs on "ubuntu-latest"
```

##### Exclusions

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

#### `require-timeouts`

Fails when a workflow job has no `timeout-minutes`. Without one, GitHub lets a
hung job run for 360 minutes (6 hours) before it cancels it, holding a runner
and billing minutes the whole time.

```yaml
jobs:
  build:
    runs-on: ubuntu-24.04
    timeout-minutes: 15
```

Jobs that call a reusable workflow (`jobs.<id>.uses`) are skipped: GitHub
doesn't accept `timeout-minutes` there, so the called workflow's own jobs need
it.

> [!WARNING]
> Timeouts set through an expression (`timeout-minutes: ${{ inputs.timeout }}`)
> can't be resolved, so the hook can't check their value. They count as set,
> and they are skipped when step and job timeouts are compared. If one resolves
> to an empty or oversized value at run time, the hook can't catch it.

Failure output

```
.github/workflows/ci.yml:12: job "build" has no timeout-minutes
```

##### Steps

A job's timeout already caps its steps. To also require `timeout-minutes` on
every step, pass `--steps`:

```yaml
    hooks:
      - id: require-timeouts
        args: [--steps]
```

```
.github/workflows/ci.yml:20: step "Build" in job "build" has no timeout-minutes
```

Steps are named by `name`, then `id`, then `uses`, else their position (`#2`).

With `--steps`, a step's timeout must also be below its job's. GitHub cancels
the job first, so a step timeout at or above the job's can never fire. Only
plain numbers are compared (see the warning above).

```
.github/workflows/ci.yml:20: step "Test" timeout-minutes 45 is not below job "build" timeout-minutes 30
```

Step timeouts may add up to more than the job's. Each one caps a single step's
worst case, and the job timeout caps the whole run.

##### Exclusions

- One job or step: put `# require-timeouts: allow` on its first line. On a job,
  this exempts its steps too.

  ```yaml
  jobs:
    soak: # require-timeouts: allow
      runs-on: ubuntu-24.04
      steps:
        - name: Soak test # require-timeouts: allow
          run: ./soak.sh
  ```

- Whole files: add `exclude:` to the hook in your `.pre-commit-config.yaml`.

## Development

Needs Go, [prek](https://prek.j178.dev), and [Task](https://taskfile.dev). Run
`prek install` once per clone: every check lives in `.pre-commit-config.yaml`
(file hygiene, zizmor, golangci-lint, this repo's own hooks on its workflows,
the manifest check, tests, `go mod tidy`) and runs on commit, and commit
messages are checked against Conventional Commits. CI runs the same hooks on
every file.

```sh
task check          # every hook on every file (what CI runs)
task test           # tests only
task next-version   # the version merging this branch would release (nothing if none; CI's release job uses it)
```

### Releases

Merging to `main` releases automatically. CI's `release` job runs
[svu](https://github.com/caarlos0/svu) over the
[Conventional Commits](https://www.conventionalcommits.org/) since the last
tag: a `fix` cuts a patch, a `feat` a minor, and a breaking change (`feat!:`,
or a `BREAKING CHANGE:` footer) a major. Anything else (`docs`, `ci`, `chore`,
...) releases nothing. The release notes are generated from the merged PR
titles, so give a squash-merged PR a conventional title.
