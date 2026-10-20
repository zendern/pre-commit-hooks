# pre-commit-hooks

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

## Hooks

### `no-latest-runners`

Fails when a workflow or composite action under `.github/` uses a floating
runner label such as `ubuntu-latest`, `macos-latest` or `windows-latest`.
GitHub repoints those labels to new images (`ubuntu-latest` moved to Ubuntu 26
from 2026-10-19), so an unpinned job changes OS without a commit. Pin a version
instead: `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, ...

It's one `pygrep` regex (multiline) starting at each `runs-on:` and covering
every form: a scalar (`runs-on: ubuntu-latest`), a flow list
(`[self-hosted, ubuntu-latest]`), a block list on the following lines, and a
`group`/`labels` object. It only walks the lines that belong to `runs-on`, so
text later in the job (`- run: echo ubuntu-latest`) doesn't match, and comments
are skipped. Labels chosen through an expression (`runs-on: ${{ matrix.os }}`)
can't be resolved and aren't checked.

### Exclusions

- One line: end it with `# no-latest-runners: allow`.

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

```sh
tests/run.sh   # runs each hook via `prek try-repo`: tests/fixtures/bad-* must fail, good-* must pass
```
