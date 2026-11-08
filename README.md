# pre-commit-hooks

[![ci](https://github.com/zendern/pre-commit-hooks/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/zendern/pre-commit-hooks/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go_1.27-00ADD8?logo=go&logoColor=white)](https://go.dev) [![pre-commit](https://img.shields.io/badge/pre--commit-hooks-FAB040?logo=precommit&logoColor=white)](https://pre-commit.com) [![prek](https://img.shields.io/badge/prek-enabled-DE611A?logo=precommit&logoColor=white)](https://github.com/j178/prek) [![Task](https://img.shields.io/badge/Task-29BEB0?logo=task&logoColor=white)](https://taskfile.dev) [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)


Shared [pre-commit](https://pre-commit.com) hooks.


They use the standard `.pre-commit-hooks.yaml` format, so they run under
pre-commit or [prek](https://prek.j178.dev) alike.

## Contents

- [Usage](#usage)
- [Hooks](#hooks)
  - [GitHub Actions](#github-actions)
    - [Warnings](#warnings)
    - [`no-latest-runners`](#no-latest-runners): pin runner labels instead of `*-latest`
    - [`require-timeouts`](#require-timeouts): require `timeout-minutes` on jobs (and steps)
    - [`require-runner-arch`](#require-runner-arch): require one CPU architecture for runners
    - [Workflow lockfile (`gh-actions-lock`)](#workflow-lockfile-gh-actions-lock): hooks that run GitHub's `gh-actions-lock` CLI
      - [`gh-actions-lock-verify-local`](#gh-actions-lock-verify-local): every action has a lockfile entry (offline, on commit)
      - [`gh-actions-lock-verify`](#gh-actions-lock-verify): the lockfile matches GitHub (manual stage, for CI)
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
      - id: require-runner-arch
        args: [--arch=arm64]
      - id: gh-actions-lock-verify-local
      - id: gh-actions-lock-verify # manual stage: run in CI with GH_TOKEN
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

Runner labels are compared without regard to case, as
[GitHub does](https://docs.github.com/en/actions/how-tos/manage-runners/self-hosted-runners/apply-labels).

All of them follow [YAML anchors and aliases](https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations)
(`&name` / `*name`), which GitHub Actions supports, wherever a value can be
one: a reused job (`alt-test: *base_job`), `runs-on`, its labels, `steps`, a
step, or `timeout-minutes`. A finding about an aliased value is reported where
it is used: the job's key for a reused job, otherwise the alias's line. An
allow marker on the anchored value covers every use of it; one on an alias's
line covers that use only. Merge keys (`<<: *name`) are not followed: GitHub
does not support them.

#### Warnings

Some things a hook can't check, such as a label set through an expression. It
prints a warning with the file and line, and doesn't fail because of it:

```
.github/workflows/ci.yml:12: warning: job "build" runs on "${{ matrix.os }}", an expression this hook can't check
```

pre-commit and prek only have pass or fail, and hide a passing hook's output, so
these hooks set `verbose: true`. To silence warnings:

- One job or step: use the hook's allow marker, described under each hook.
- A whole hook: set `verbose: false` on it in your `.pre-commit-config.yaml`.

#### `no-latest-runners`

Fails when a workflow job runs on a floating runner label such as
`ubuntu-latest`, `macos-latest` or `windows-latest`. GitHub repoints those
labels to new images (ex. `ubuntu-latest` moved to Ubuntu 26 starting on 2026-10-19), so
an unpinned job changes OS without a commit potentially breaking builds since underlying tooling changes.

Pin a version instead:
`ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, ...

It parses each workflow and reads `jobs.<id>.runs-on` in every form: a label, a
list of labels, or a `group`/`labels` object.

Labels set through an expression (`runs-on: ${{ matrix.os }}`) can't be resolved,
so they get a [warning](#warnings).

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

Timeouts set through an expression (`timeout-minutes: ${{ inputs.timeout }}`)
can't be resolved, so they get a [warning](#warnings). They count as set, and
they are skipped when step and job timeouts are compared. If one resolves to an
empty or oversized value at run time, the hook can't catch it.

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

#### `require-runner-arch`

Fails when a workflow job runs on a different CPU architecture from the one you
require. `--arch` is required, and takes `arm64` or `x64`.

```yaml
    hooks:
      - id: require-runner-arch
        args: [--arch=arm64]
```

A standard arm64 Linux runner costs $0.005 per minute against $0.006 for x64, so
`--arch=arm64` makes included usage in a private repository last 20% longer. See
GitHub's [runner pricing](https://docs.github.com/en/billing/reference/actions-runner-pricing).
`--arch=x64` suits a project that ships amd64 and wants its CI to match.

The hook knows GitHub's own runner labels:

| Label | Architecture |
| --- | --- |
| `ubuntu-*-arm`, `windows-*-arm` | arm64 |
| other `ubuntu-*`, `windows-*` | x64 |
| `macos-*-intel`, `macos-*-large`, `macos-13` | x64 |
| other `macos-*` | arm64 |

It can't tell the architecture of any other label, so by default a job with only
those labels, or with the `self-hosted` label, passes with a
[warning](#warnings).

Labels set through an expression (`runs-on: ${{ matrix.os }}`) can't be resolved,
so they get a warning too, even with `--custom-label-matcher` set.

Failure output

```
.github/workflows/ci.yml:12: job "build" runs on "ubuntu-24.04"
```

##### Custom runners

If you have larger or self-hosted runners with your own labels, tell the hook
which ones are on the required architecture with a
[Go regular expression](https://pkg.go.dev/regexp/syntax):

```yaml
    hooks:
      - id: require-runner-arch
        args: [--arch=arm64, --custom-label-matcher=^arc-arm-]
```

The expression is searched for in each label, so anchor it. It ignores case,
because GitHub's labels do. With this flag set:

- A job passes when any of its labels matches.
- A job with only custom labels fails when none of them match, and self-hosted
  jobs no longer just warn.

##### Exclusions

Some jobs have to stay on another architecture, such as one that builds
`linux/amd64` images without emulation.

- One job: put `# require-runner-arch: allow` on its `runs-on` line (or on a
  list item's line).

  ```yaml
  runs-on: ubuntu-24.04 # require-runner-arch: allow
  ```

- Whole files: add `exclude:` to the hook in your `.pre-commit-config.yaml`.

#### Workflow lockfile (`gh-actions-lock`)

These hooks wrap GitHub's [`gh-actions-lock`](https://github.com/github/gh-actions-lock)
CLI, the tool behind the `gh actions-lock` extension, rather than parsing
workflows themselves, so the YAML-anchor and [warnings](#warnings) notes above
don't apply to them. They check the workflow lockfile
`.github/workflows/actions.lock`, which pins every action a workflow uses,
including the actions those actions call, to a commit.

Both only read. Regenerate the lockfile yourself after editing a `uses:` line:

```sh
gh actions-lock
```

##### `gh-actions-lock-verify-local`

Fails when an action used in `.github/workflows/` has no entry in the
lockfile. It runs `gh-actions-lock --verify-local`: offline, with no token.

It runs when a workflow, the lockfile, or an `action.yml` changes (in-repo
actions reached through `$/` are locked too). It only checks that every action
has an entry, not that the entries are genuine; [`gh-actions-lock-verify`](#gh-actions-lock-verify)
does that.

Failure output

```
1 of 2 workflows failed: 1 not-pinned
Run without --verify-local to resolve and pin missing actions.
```

##### `gh-actions-lock-verify`

Fails when an entry in `.github/workflows/actions.lock` doesn't match GitHub:
a tag that moved, a locked commit no longer in the action's history, a changed
owner, or an unused entry. It runs `gh-actions-lock --verify`, which resolves
every entry as the runner does at job start.

It needs `GH_TOKEN` and the network, so it is in the `manual` stage and never
runs on commit. Run it in CI:

```yaml
      - name: Verify the workflow lockfile
        env:
          GH_TOKEN: ${{ github.token }}
        run: prek run --hook-stage manual gh-actions-lock-verify --all-files
```

Naming the hook id makes that command fail when a repo hasn't configured the
hook, so CI can't skip the check silently. Locally:
`GH_TOKEN=$(gh auth token) prek run --hook-stage manual gh-actions-lock-verify --all-files`.

Failure output

```
1 of 2 workflows failed
✗ Pinned commit not in upstream history actions/create-github-app-token@v3.3.0
  locked commit 64ab729a6294 is not in actions/create-github-app-token's history: it was force-pushed away, or the lockfile was edited; v3.3.0 currently points at 64ab729a6294
  ! verify it, then run `gh actions-lock --accept-moved`
  ↳ .github/workflows/ruleset-drift.yml
```

##### Choosing when they run

The two hooks are separate so each is safe to list with no extra config:
`gh-actions-lock-verify-local` on commit, `gh-actions-lock-verify` only when
asked. Neither runs `gh-actions-lock` without a flag, which rewrites workflows
and the lockfile.

To run either one at another point, override `stages`. For example, to check
provenance before every push, with `GH_TOKEN` set in your shell:

```yaml
    hooks:
      - id: gh-actions-lock-verify-local
      - id: gh-actions-lock-verify
        stages: [pre-push]
```

##### gh-actions-lock version

Both hooks build `gh-actions-lock` from its Go module at the version in `.pre-commit-hooks.yaml`, so neither needs
`gh` or the `gh actions-lock` extension installed. The Go checksum database
rejects a tag that moved after it was first published.

To run a different version, for example to try a new release early, override
`additional_dependencies` on both hooks, so the commit and CI checks agree. The
list replaces the hook's own. Pin the release's commit SHA with its tag in a
comment; Go resolves a tagged commit to that version:

```yaml
    hooks:
      - id: gh-actions-lock-verify-local
        additional_dependencies:
          - github.com/github/gh-actions-lock/cmd/gh-actions-lock@759f9c2d44e8599fa5b29224e5a30c58b3555211 # v0.4.0
      - id: gh-actions-lock-verify
        additional_dependencies:
          - github.com/github/gh-actions-lock/cmd/gh-actions-lock@759f9c2d44e8599fa5b29224e5a30c58b3555211 # v0.4.0
```

Renovate doesn't bump an override, and a repo that keeps one drifts from the
version every other repo runs, so remove it once this repository's pin catches
up.

## Development

Install Go 1.21 or newer, then [Task](https://taskfile.dev) at the
`TASK_VERSION` pinned in `Taskfile.yml`:

```sh
go install github.com/go-task/task/v3/cmd/task@v"$(sed -n 's/^  TASK_VERSION: *//p' Taskfile.yml)"
task setup   # alias: task install-tools
```

`task setup` installs every other tool into `./bin` at the versions pinned in
`Taskfile.yml` ([prek](https://prek.j178.dev), golangci-lint, govulncheck,
go-licenses, go-test-coverage, gremlins, [ratchet](https://github.com/sethvargo/ratchet)),
then the git hooks. Run it in the
main checkout, not a worktree: worktrees share its hooks, and the hooks run the
prek in the `./bin` of whichever checkout installed them. Every `go` command
Task runs uses the exact toolchain in `go.mod` (`GOTOOLCHAIN`), which Go
downloads when yours is a different version.

Every check lives in `.pre-commit-config.yaml` and runs on commit: file
hygiene, zizmor, golangci-lint (with gofumpt and goimports), this repo's own
hooks on its workflows, the manifest check, tests with a 95% coverage gate
(`.testcoverage.yml`), `go mod tidy`, govulncheck, and go-licenses. Commit
messages are checked against Conventional Commits. CI runs the same hooks on
every file. Mutation testing (gremlins, 95% efficacy and mutant coverage in
`.gremlins.yaml`) is too slow for a hook: the `mutation` workflow runs it on
the packages a PR changes, and on the whole module weekly. A tagless
`switch { case cond: }` fails lint (`tools/lint/rules.go`), because gremlins
cannot mutate case conditions; write `if` statements.

Every action in a workflow must be pinned to a commit SHA: zizmor's
`unpinned-uses` audit fails a tag like `actions/checkout@v5`, on commit and in
CI. After adding or changing a `uses:` line, run `task pin` (ratchet), which
rewrites it to `actions/checkout@<sha> # ratchet:actions/checkout@v5` and leaves
lines already pinned alone. It looks refs up through the GitHub API, so set
`GITHUB_TOKEN` (for example `GITHUB_TOKEN=$(gh auth token) task pin`) to avoid
the anonymous rate limit. Renovate then keeps the pins current; it reads both
the `# ratchet:` and `# vX.Y.Z` comment styles.

```sh
task check          # every hook on every file (what CI runs)
task test           # tests with the race detector
task cover          # tests with the coverage gate
task lint           # golangci-lint
task fmt            # gofumpt and goimports
task mutate         # mutation testing (mutate-diff BASE=<ref> for changed packages)
task vuln           # govulncheck
task licenses       # dependency license check
task pin            # pin new or changed GitHub Actions to a commit SHA (ratchet)
task pin-check      # fail on any unpinned action or container image (ratchet lint)
task next-version   # the version merging this branch would release (nothing if none; CI's release job uses it)
```

### Releases

Merging to `main` releases automatically. CI's `release` job runs
[svu](https://github.com/caarlos0/svu) over the
[Conventional Commits](https://www.conventionalcommits.org/) since the last
tag: a `fix` cuts a patch, a `feat` a minor, and a breaking change (`feat!:`,
or a `BREAKING CHANGE:` footer) a major. Any other change to the code a
consuming repo runs (`cmd/`, `internal/`, `go.mod`, `go.sum`,
`.pre-commit-hooks.yaml`, tests and test fixtures aside), such as a `refactor`
or `perf`, cuts a patch too. Anything else (`docs`, `ci`, `chore`, tests)
releases nothing. The release notes are generated from the merged PR titles, so
give a squash-merged PR a conventional title.
