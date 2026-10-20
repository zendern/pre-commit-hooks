package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string // "job label line"
	}{
		{"scalar", `
jobs:
  a:
    runs-on: ubuntu-latest
`, []string{"a ubuntu-latest 4"}},
		{"quoted with comment", `
jobs:
  a:
    runs-on: "macos-latest" # build on mac
`, []string{"a macos-latest 4"}},
		{"flow list", `
jobs:
  a:
    runs-on: [self-hosted, windows-latest]
`, []string{"a windows-latest 4"}},
		{"block list", `
jobs:
  a:
    runs-on:
      - self-hosted
      - ubuntu-latest
    steps:
      - run: echo hi
`, []string{"a ubuntu-latest 6"}},
		{"group labels", `
jobs:
  a:
    runs-on:
      group: big
      labels: ubuntu-latest
`, []string{"a ubuntu-latest 6"}},
		{"several jobs", `
jobs:
  a:
    runs-on: ubuntu-latest
  b:
    runs-on: ubuntu-24.04
  c:
    runs-on: macos-latest
`, []string{"a ubuntu-latest 4", "c macos-latest 8"}},
		{"pinned", `
jobs:
  a:
    runs-on: ubuntu-24.04
  b:
    runs-on: [self-hosted, ubuntu-24.04-arm]
`, nil},
		{"latest only in other keys, steps and comments", `
env:
  RUNNER: ubuntu-latest
jobs:
  a:
    # was ubuntu-latest
    runs-on: ubuntu-24.04 # was ubuntu-latest
    container: node:latest
    env:
      labels: ubuntu-latest
    steps:
      - run: echo ubuntu-latest
`, nil},
		{"expression", `
jobs:
  a:
    runs-on: ${{ matrix.os }}
`, nil},
		{"allow marker", `
jobs:
  a:
    runs-on: ubuntu-latest # no-latest-runners: allow
  b:
    runs-on:
      - self-hosted
      - macos-latest # no-latest-runners: allow
  c:
    runs-on: [self-hosted, windows-latest] # no-latest-runners: allow
  d:
    runs-on: # no-latest-runners: allow
      group: big
      labels: ubuntu-latest
`, nil},
		{"reusable workflow job without runs-on", `
jobs:
  a:
    uses: ./.github/workflows/ci.yml
`, nil},
		{"empty file", ``, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := check(strings.NewReader(tt.yaml))
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			var got []string
			for _, f := range findings {
				got = append(got, fmt.Sprint(f.job, " ", f.label, " ", f.line))
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckInvalidYAML(t *testing.T) {
	if _, err := check(strings.NewReader("jobs: [unclosed")); err == nil {
		t.Fatal("want an error for invalid YAML")
	}
}
