package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

const hint = "Pin runner labels to a version (e.g. ubuntu-24.04) instead of *-latest.\n"

func TestRunPinned(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{filepath.Join("testdata", "pinned.yml")}, &stdout, &stderr)

	if code != 0 || stdout.String() != "" || stderr.String() != "" {
		t.Fatalf("code %d, stdout %q, stderr %q; want 0 and no output", code, stdout.String(), stderr.String())
	}
}

func TestRunNoFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("code %d, want 0", code)
	}
}

func TestRunFloating(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join("testdata", "several-jobs.yml")

	code := run([]string{path}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	want := path + `:3: job "a" runs on "ubuntu-latest"` + "\n" +
		path + `:7: job "c" runs on "macos-latest"` + "\n"
	if stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
	if stderr.String() != hint {
		t.Errorf("stderr %q, want %q", stderr.String(), hint)
	}
}

func TestRunMissingFileStillChecksTheRest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join("testdata", "missing.yml")
	floating := filepath.Join("testdata", "scalar.yml")

	code := run([]string{missing, floating}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), missing+": open ") || !strings.HasSuffix(stderr.String(), hint) {
		t.Errorf("stderr %q, want the open error, then the hint", stderr.String())
	}
	if want := floating + `:3: job "a" runs on "ubuntu-latest"` + "\n"; stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
}

func TestRunMissingFileAlone(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{filepath.Join("testdata", "missing.yml")}, &stdout, &stderr); code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
}

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
