package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	tests := []struct {
		file string   // in testdata/
		want []string // "job label line"
	}{
		{"scalar.yml", []string{"a ubuntu-latest 3"}},
		{"quoted-with-comment.yml", []string{"a macos-latest 3"}},
		{"flow-list.yml", []string{"a windows-latest 3"}},
		{"block-list.yml", []string{"a ubuntu-latest 5"}},
		{"group-labels.yml", []string{"a ubuntu-latest 5"}},
		{"several-jobs.yml", []string{"a ubuntu-latest 3", "c macos-latest 7"}},
		{"pinned.yml", nil},
		{"latest-outside-runs-on.yml", nil},
		{"expression.yml", nil},
		{"allow-marker.yml", nil},
		{"reusable-workflow-call.yml", nil},
		{"empty.yml", nil},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			findings, err := checkFile(filepath.Join("testdata", tt.file))
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
