# prek-hooks

Shared [prek](https://prek.j178.dev) / [pre-commit](https://pre-commit.com)
hooks for zendern's repositories. Public, so CI (and public repos) can fetch it
without credentials.

## Usage

```yaml
repos:
  - repo: https://github.com/zendern/prek-hooks
    rev: <full commit sha>  # frozen: v1.0.0
    hooks:
      - id: no-latest-runners
```

Freeze `rev` to a commit SHA (`prek autoupdate --freeze`); Renovate's
pre-commit manager bumps it.

## Hooks

### `no-latest-runners`

Fails when a workflow or composite action under `.github/` uses a floating
runner label such as `ubuntu-latest`, `macos-latest` or `windows-latest`.
GitHub repoints those labels to new images (`ubuntu-latest` moved to Ubuntu 26
from 2026-10-19), so an unpinned job changes OS without a commit. Pin a version
instead: `ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-15`, ...

It checks every `runs-on` form: a scalar, a flow list
(`[self-hosted, ubuntu-latest]`), a block list, and `group`/`labels` objects.
Comments are ignored. A label chosen through an expression
(`runs-on: ${{ matrix.os }}`) can't be resolved statically and isn't checked.

## Development

```sh
tests/run.sh   # every tests/fixtures/bad-* must fail, every good-* must pass
```
