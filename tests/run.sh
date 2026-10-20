#!/usr/bin/env bash
# Every fixture named bad-* must fail the hook and every good-* must pass.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
hook="$root/hooks/no-latest-runners.sh"
failures=0

for f in "$root"/tests/fixtures/*.yml; do
  name="$(basename "$f")"
  if "$hook" "$f" >/dev/null 2>&1; then got=pass; else got=fail; fi
  case "$name" in
    bad-*) want=fail ;;
    good-*) want=pass ;;
    *) echo "unexpected fixture name: $name" >&2; exit 2 ;;
  esac
  if [[ "$got" == "$want" ]]; then
    echo "ok   $name ($got)"
  else
    echo "FAIL $name: wanted $want, got $got"
    failures=$((failures + 1))
  fi
done

exit "$failures"
