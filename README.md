# pre-commit-hooks

Shared [pre-commit](https://pre-commit.com) hooks.
They use the standard `.pre-commit-hooks.yaml` format, so they run under
pre-commit or [prek](https://prek.j178.dev) alike. 

## Usage

In `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/zendern/pre-commit-hooks
    rev: <full commit sha>  # frozen: v1.0.0
    hooks:
      - id: no-latest-runners
```

> [!NOTE]
> Suggestion:  Freeze `rev` to a commit SHA (`prek autoupdate --freeze` or
> `pre-commit autoupdate --freeze`); You can also use Renovate's pre-commit manager to bump it.

The hooks are Go programs (`language: golang`). pre-commit and prek build them
with `go install` on first use, and download a Go toolchain if none is
installed.

## Hooks

### `no-latest-runners`

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
