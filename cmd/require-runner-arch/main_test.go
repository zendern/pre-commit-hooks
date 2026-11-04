package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	armHint    = "Use an arm64 runner label (e.g. ubuntu-24.04-arm), or put `# require-runner-arch: allow` on the runs-on line of a job that has to stay on another architecture.\n"
	x64Hint    = "Use an x64 runner label (e.g. ubuntu-24.04), or put `# require-runner-arch: allow` on the runs-on line of a job that has to stay on another architecture.\n"
	customHint = "A custom runner counts as arm64 only when one of its labels matches --custom-label-matcher \"^arc-arm-\".\n"
)

func TestRunArm(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := run([]string{"--arch=arm64", filepath.Join("testdata", "arm.yml")}, &stdout, &stderr)

	if code != 0 || stdout.String() != "" || stderr.String() != "" {
		t.Fatalf("code %d, stdout %q, stderr %q; want 0 and no output", code, stdout.String(), stderr.String())
	}
}

func TestRunNoFiles(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{"--arch=arm64"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code %d, want 0", code)
	}
}

func TestRunWrongArch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join("testdata", "several-jobs.yml")

	code := run([]string{"--arch=arm64", path}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	want := path + `:3: job "a" runs on "ubuntu-24.04"` + "\n" +
		path + `:7: job "c" runs on "windows-2025"` + "\n"
	if stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
	if stderr.String() != armHint {
		t.Errorf("stderr %q, want %q", stderr.String(), armHint)
	}
}

func TestRunWrongArchX64(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join("testdata", "several-jobs.yml")

	code := run([]string{"--arch=x64", path}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	if want := path + `:5: job "b" runs on "ubuntu-24.04-arm"` + "\n"; stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
	if stderr.String() != x64Hint {
		t.Errorf("stderr %q, want %q", stderr.String(), x64Hint)
	}
}

func TestRunMissingFileStillChecksTheRest(t *testing.T) {
	var stdout, stderr bytes.Buffer
	missing := filepath.Join("testdata", "missing.yml")
	wrong := filepath.Join("testdata", "scalar.yml")

	code := run([]string{"--arch=arm64", missing, wrong}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	if !strings.HasPrefix(stderr.String(), missing+": open ") || !strings.HasSuffix(stderr.String(), armHint) {
		t.Errorf("stderr %q, want the open error, then the hint", stderr.String())
	}
	if want := wrong + `:3: job "a" runs on "ubuntu-24.04"` + "\n"; stdout.String() != want {
		t.Errorf("stdout %q, want %q", stdout.String(), want)
	}
}

func TestRunMissingFileAlone(t *testing.T) {
	var stdout, stderr bytes.Buffer

	if code := run([]string{"--arch=arm64", filepath.Join("testdata", "missing.yml")}, &stdout, &stderr); code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
}

func TestRunCustomLabelMatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	path := filepath.Join("testdata", "custom-labels.yml")

	code := run([]string{"--arch=arm64", "--custom-label-matcher", "^arc-arm-", path}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("code %d, want 1", code)
	}
	if want := path + `:5: job "b" runs on "arc-x64-small"` + "\n"; !strings.HasPrefix(stdout.String(), want) {
		t.Errorf("stdout %q, want it to start with %q", stdout.String(), want)
	}
	if stderr.String() != armHint+customHint {
		t.Errorf("stderr %q, want %q", stderr.String(), armHint+customHint)
	}
}

func TestRunBadFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string // in stderr
	}{
		{"missing arch", nil, "--arch is required"},
		{"unknown arch", []string{"--arch=sparc"}, "must be arm64 or x64"},
		{"invalid expression", []string{"--arch=arm64", "--custom-label-matcher=("}, "-custom-label-matcher"},
		{"unknown flag", []string{"--bogus"}, "flag provided but not defined: -bogus"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer

			if code := run(tt.args, &stdout, &stderr); code != 2 {
				t.Fatalf("code %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Errorf("stderr %q, want it to contain %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	arc := regexp.MustCompile("^arc-arm-")
	arcX64 := regexp.MustCompile("^arc-x64-")
	tests := []struct {
		file   string // in testdata/
		arch   string
		custom *regexp.Regexp
		want   []string // "job label line"
	}{
		{"scalar.yml", arm64, nil, []string{"a ubuntu-24.04 3"}},
		{"quoted-with-comment.yml", arm64, nil, []string{"a windows-2025 3"}},
		{"flow-list.yml", arm64, nil, []string{"a ubuntu-24.04 3"}},
		{"block-list.yml", arm64, nil, []string{"a ubuntu-24.04 5"}},
		{"group-labels.yml", arm64, nil, []string{"a ubuntu-24.04 5"}},
		{"several-jobs.yml", arm64, nil, []string{"a ubuntu-24.04 3", "c windows-2025 7"}},
		{"x64-labels.yml", arm64, nil, []string{
			"a ubuntu-latest 3",
			"b ubuntu-slim 5",
			"c windows-latest 7",
			"d macos-15-intel 9",
			"e macos-15-large 11",
			"f macos-13 13",
		}},
		{"arm.yml", arm64, nil, nil},
		{"x64-outside-runs-on.yml", arm64, nil, nil},
		{"expression.yml", arm64, nil, nil},
		{"allow-marker.yml", arm64, nil, nil},
		{"reusable-workflow-call.yml", arm64, nil, nil},
		{"empty.yml", arm64, nil, nil},
		// --arch=x64 turns the same labels around.
		{"x64-labels.yml", x64, nil, nil},
		{"arm.yml", x64, nil, []string{
			"a ubuntu-24.04-arm 3",
			"b windows-11-arm 5",
			"c macos-15 7",
			"d macos-latest 9",
			"e macos-15-xlarge 11",
			"h macos-13-xlarge 17",
		}},
		// Self-hosted jobs are skipped when there's no way to judge their labels.
		// An exact label only: a lookalike doesn't count.
		{"self-hosted.yml", arm64, nil, []string{"d ubuntu-24.04 11"}},
		// Without --custom-label-matcher, custom labels pass.
		{"custom-labels.yml", arm64, nil, []string{"f ubuntu-24.04 13"}},
		// With it, a match passes the whole job, and a job with only custom
		// labels and no match is reported once, with every label.
		{"custom-labels.yml", arm64, arc, []string{
			"b arc-x64-small 5",
			"d self-hosted, linux, x64 9",
			"f ubuntu-24.04 13",
			"j ubuntu-24.04 21",
			"m arc-x64-big 31",
		}},
		{"custom-labels.yml", x64, arcX64, []string{
			"a arc-arm-small 3",
			"c self-hosted, linux, arc-arm-big 7",
			"d self-hosted, linux, x64 9",
			"e ubuntu-24.04-arm 11",
			"g macos-15 15",
		}},
		// Aliases are followed and reported where they are used: the alias's
		// line, or the job's key for a job reused whole.
		{"alias-scalar.yml", arm64, nil, []string{"a ubuntu-24.04 5"}},
		{"alias-reused-job.yml", arm64, nil, []string{"test ubuntu-24.04 3", "alt-test ubuntu-24.04 4"}},
		{"alias-list-item.yml", arm64, nil, []string{"a windows-2025 7", "b windows-2025 9"}},
		{"alias-labels.yml", arm64, nil, []string{"a ubuntu-24.04 5", "b ubuntu-24.04 9", "c ubuntu-24.04 11"}},
		// A marker on the alias exempts that use; one on the anchor, every use.
		{"alias-allow.yml", arm64, nil, []string{"c ubuntu-24.04 10", "g windows-2025 15"}},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s %v", tt.file, tt.arch, tt.custom), func(t *testing.T) {
			findings, err := checkFile(filepath.Join("testdata", tt.file), policy{arch: tt.arch, custom: tt.custom})
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
	if _, err := check(strings.NewReader("jobs: [unclosed"), policy{arch: arm64}); err == nil {
		t.Fatal("want an error for invalid YAML")
	}
}
