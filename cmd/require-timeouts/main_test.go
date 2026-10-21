package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		name  string
		steps bool
		yaml  string
		want  []string // "job step line"; step is "-" for a job finding
	}{
		{"job missing timeout", false, `
jobs:
  a:
    runs-on: ubuntu-24.04
    steps:
      - run: echo hi
`, []string{"a - 3"}},
		{"job with timeout", false, `
jobs:
  a:
    runs-on: ubuntu-24.04
    timeout-minutes: 10
`, nil},
		{"expression timeout", false, `
jobs:
  a:
    timeout-minutes: ${{ inputs.timeout }}
`, nil},
		{"empty timeout", false, `
jobs:
  a:
    timeout-minutes:
`, []string{"a - 3"}},
		{"several jobs", false, `
jobs:
  a:
    runs-on: ubuntu-24.04
  b:
    runs-on: ubuntu-24.04
    timeout-minutes: 5
  c:
    runs-on: ubuntu-24.04
`, []string{"a - 3", "c - 8"}},
		{"reusable workflow call", false, `
jobs:
  a:
    uses: ./.github/workflows/build.yml
`, nil},
		{"steps not checked by default", false, `
jobs:
  a:
    timeout-minutes: 10
    steps:
      - run: echo hi
`, nil},
		{"steps checked", true, `
jobs:
  a:
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@v5
      - name: Build
        run: make
      - run: make test
        timeout-minutes: 5
      - id: lint
        run: make lint
`, []string{`a "actions/checkout@v5" 6`, `a "Build" 7`, `a "lint" 11`}},
		{"step falls back to its position", true, `
jobs:
  a:
    timeout-minutes: 10
    steps:
      - run: echo hi
`, []string{"a #1 6"}},
		{"job and steps both missing", true, `
jobs:
  a:
    steps:
      - run: echo hi
`, []string{"a - 3", "a #1 5"}},
		{"allow marker", true, `
jobs:
  a: # require-timeouts: allow
    steps:
      - run: echo hi
  b:
    timeout-minutes: 10
    steps:
      - run: ./long-soak-test # require-timeouts: allow
      - name: Deploy # require-timeouts: allow
        run: make deploy
`, nil},
		{"no jobs", false, `
on: push
`, nil},
		{"empty file", false, ``, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := check(strings.NewReader(tt.yaml), tt.steps)
			if err != nil {
				t.Fatalf("check: %v", err)
			}
			var got []string
			for _, f := range findings {
				step := "-"
				if f.step != "" {
					step = f.step
				}
				got = append(got, fmt.Sprint(f.job, " ", step, " ", f.line))
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckInvalidYAML(t *testing.T) {
	if _, err := check(strings.NewReader("jobs: [unclosed"), false); err == nil {
		t.Fatal("want an error for invalid YAML")
	}
}
