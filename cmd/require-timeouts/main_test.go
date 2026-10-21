package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		file  string // in testdata/
		steps bool
		want  []string // "job step line"; step is "-" for a job finding
	}{
		{"job-missing-timeout.yml", false, []string{"a - 2"}},
		{"job-with-timeout.yml", false, nil},
		{"expression-timeout.yml", false, nil},
		{"empty-timeout.yml", false, []string{"a - 2"}},
		{"several-jobs.yml", false, []string{"a - 2", "c - 7"}},
		{"reusable-workflow-call.yml", false, nil},
		{"steps-without-flag.yml", false, nil},
		{"steps-checked.yml", true, []string{`a "actions/checkout@v5" 5`, `a "Build" 6`, `a "lint" 10`}},
		{"step-position-label.yml", true, []string{"a #1 5"}},
		{"job-and-steps-both-missing.yml", true, []string{"a - 2", "a #1 4"}},
		{"allow-marker.yml", true, nil},
		{"no-jobs.yml", false, nil},
		{"empty.yml", false, nil},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s steps=%t", tt.file, tt.steps), func(t *testing.T) {
			findings, err := checkFile(filepath.Join("testdata", tt.file), tt.steps)
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
