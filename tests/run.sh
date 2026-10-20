#!/usr/bin/env bash
# Runs the hook the way a consumer repo does (prek try-repo): each
# tests/fixtures/bad-* workflow must fail it, each good-* must pass.
set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
rc=0

for fixture in "$root"/tests/fixtures/*.yml; do
  name="$(basename "$fixture")"
  repo="$(mktemp -d "${TMPDIR:-/tmp}/hook-test.XXXXXX")"
  mkdir -p "$repo/.github/workflows"
  cp "$fixture" "$repo/.github/workflows/ci.yml"
  git -C "$repo" init -q
  git -C "$repo" add -A # --all-files only sees tracked files

  if prek try-repo "$root" no-latest-runners --all-files --cd "$repo" >/dev/null 2>&1; then
    result=pass
  else
    result=fail
  fi
  [[ "$name" == bad-* ]] && want=fail || want=pass

  if [[ "$result" == "$want" ]]; then echo "ok   $name"; else echo "FAIL $name: wanted $want"; rc=1; fi
  rm -rf "$repo"
done

exit "$rc"
