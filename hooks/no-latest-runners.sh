#!/usr/bin/env bash
# Fail when a workflow or composite action uses a floating runner label such as
# ubuntu-latest, macos-latest or windows-latest. GitHub repoints those labels to
# new images (ubuntu-latest moved to Ubuntu 26 from 2026-10-19), so an
# unpinned job changes OS under you without a commit. Pin a version instead:
# ubuntu-24.04, ubuntu-24.04-arm, macos-15, ...
#
# Handles all three runs-on forms:
#   runs-on: ubuntu-latest
#   runs-on: [self-hosted, ubuntu-latest]
#   runs-on:
#     - ubuntu-latest
# and group/labels objects (`labels: ubuntu-latest` under runs-on). A label
# chosen through an expression (runs-on: ${{ matrix.os }}) can't be resolved
# statically and is not checked.
#
# Usage: no-latest-runners.sh FILE...   (prek/pre-commit passes the files)
set -euo pipefail

status=0
for file in "$@"; do
  # Prints file:line: text for each offending line. Inside a runs-on block
  # (a runs-on: key with nothing after it), every more-indented line belongs
  # to it until indentation returns to the key's level.
  if ! awk -v file="$file" '
    function indent(s) { match(s, /^[ \t]*/); return RLENGTH }
    function report() { printf "%s:%d: %s\n", file, NR, $0; bad = 1 }
    {
      line = $0
      sub(/[ \t]+#.*$/, "", line)          # drop trailing comments
      if (line ~ /^[ \t]*#/ || line ~ /^[ \t]*$/) next
      if (block && indent(line) <= block_indent) block = 0
      if (block) { if (line ~ /-latest([^A-Za-z0-9_.-]|$)/) report(); next }
      if (line ~ /^[ \t]*runs-on:/) {
        rest = line; sub(/^[ \t]*runs-on:[ \t]*/, "", rest)
        if (rest == "") { block = 1; block_indent = indent(line) }
        else if (rest ~ /-latest([^A-Za-z0-9_.-]|$)/) report()
      }
    }
    END { exit bad }' "$file"; then
    status=1
  fi
done

if [[ "$status" -ne 0 ]]; then
  echo "Pin runner labels to a version (e.g. ubuntu-24.04) instead of *-latest." >&2
fi
exit "$status"
