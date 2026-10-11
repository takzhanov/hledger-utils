package ui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseDateInput(t *testing.T) {
	reference := time.Date(2026, time.September, 11, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		input        string
		want         string
		wantInferred bool
	}{
		{name: "day", input: "7", want: "2026-09-07", wantInferred: true},
		{name: "two digit day", input: "07", want: "2026-09-07", wantInferred: true},
		{name: "month and day", input: "12-31", want: "2026-12-31", wantInferred: true},
		{name: "full date", input: "2025-12-31", want: "2025-12-31", wantInferred: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, inferred, err := ParseDateInput(tt.input, reference)
			if err != nil {
				t.Fatalf("ParseDateInput() error = %v", err)
			}
			if formatted := got.Format("2006-01-02"); formatted != tt.want {
				t.Fatalf("ParseDateInput() = %q, want %q", formatted, tt.want)
			}
			if inferred != tt.wantInferred {
				t.Fatalf("ParseDateInput() inferred = %v, want %v", inferred, tt.wantInferred)
			}
		})
	}
}

func TestParseDateInputRejectsInvalidDates(t *testing.T) {
	reference := time.Date(2026, time.September, 11, 0, 0, 0, 0, time.UTC)
	for _, input := range []string{"", "09-31", "9-07", "2026-02-29", "tomorrow"} {
		t.Run(input, func(t *testing.T) {
			if _, _, err := ParseDateInput(input, reference); err == nil {
				t.Fatalf("ParseDateInput(%q) unexpectedly succeeded", input)
			}
		})
	}
}

func TestAskDatePrintsResolvedAbbreviatedDate(t *testing.T) {
	reference := time.Date(2026, time.September, 11, 0, 0, 0, 0, time.UTC)
	var output bytes.Buffer
	prompter := NewPrompter(bytes.NewBufferString("12-31\n"), &output)

	got, err := prompter.AskDate("Date", reference)
	if err != nil {
		t.Fatalf("AskDate() error = %v", err)
	}
	if formatted := got.Format("2006-01-02"); formatted != "2026-12-31" {
		t.Fatalf("AskDate() = %q, want %q", formatted, "2026-12-31")
	}
	if want := "Date [2026-09-11]: Resolved date: 2026-12-31\n"; output.String() != want {
		t.Fatalf("AskDate() output = %q, want %q", output.String(), want)
	}
}

func TestAskDateDoesNotRepeatFullDate(t *testing.T) {
	reference := time.Date(2026, time.September, 11, 0, 0, 0, 0, time.UTC)
	var output bytes.Buffer
	prompter := NewPrompter(bytes.NewBufferString("2025-12-31\n"), &output)

	if _, err := prompter.AskDate("Date", reference); err != nil {
		t.Fatalf("AskDate() error = %v", err)
	}
	if want := "Date [2026-09-11]: "; output.String() != want {
		t.Fatalf("AskDate() output = %q, want %q", output.String(), want)
	}
}

func TestEnsureFZFAcceptsInstalledBinary(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, filepath.Join(binDir, "fzf"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", binDir)
	var output bytes.Buffer

	if err := EnsureFZF(&output); err != nil {
		t.Fatalf("EnsureFZF() error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("EnsureFZF() output = %q, want empty", output.String())
	}
}

func TestEnsureFZFInstallsWithHomebrew(t *testing.T) {
	binDir := t.TempDir()
	brew := "#!/bin/sh\nprintf '#!/bin/sh\\nexit 0\\n' > \"$FAKE_BIN/fzf\"\n/bin/chmod +x \"$FAKE_BIN/fzf\"\n"
	writeExecutable(t, filepath.Join(binDir, "brew"), brew)
	t.Setenv("PATH", binDir)
	t.Setenv("FAKE_BIN", binDir)
	var output bytes.Buffer

	if err := EnsureFZF(&output); err != nil {
		t.Fatalf("EnsureFZF() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(binDir, "fzf")); err != nil {
		t.Fatalf("fzf was not installed: %v", err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestAskDateRetriesAfterInvalidInput(t *testing.T) {
	reference := time.Date(2026, time.September, 11, 0, 0, 0, 0, time.UTC)
	var output bytes.Buffer
	prompter := NewPrompter(bytes.NewBufferString("tomorrow\n09-31\n12-31\n"), &output)

	got, err := prompter.AskDate("Date", reference)
	if err != nil {
		t.Fatalf("AskDate() error = %v", err)
	}
	if formatted := got.Format("2006-01-02"); formatted != "2026-12-31" {
		t.Fatalf("AskDate() = %q, want %q", formatted, "2026-12-31")
	}
	if want := 2; strings.Count(output.String(), "date must use DD, MM-DD, or YYYY-MM-DD") != want {
		t.Fatalf("AskDate() output = %q, want %d format errors", output.String(), want)
	}
}

func TestAskRequiredStringRetriesUntilAnswered(t *testing.T) {
	var output bytes.Buffer
	prompter := NewPrompter(bytes.NewBufferString("\n   \nlunch\n"), &output)

	got, err := prompter.AskRequiredString("Description")
	if err != nil {
		t.Fatalf("AskRequiredString() error = %v", err)
	}
	if got != "lunch" {
		t.Fatalf("AskRequiredString() = %q, want %q", got, "lunch")
	}
	if want := 2; strings.Count(output.String(), "Description is required") != want {
		t.Fatalf("AskRequiredString() output = %q, want %d reminders", output.String(), want)
	}
}

func TestAskRequiredStringStopsAtEndOfInput(t *testing.T) {
	var output bytes.Buffer
	prompter := NewPrompter(bytes.NewBufferString(""), &output)

	if _, err := prompter.AskRequiredString("Description"); err == nil {
		t.Fatal("AskRequiredString() should fail when input ends")
	}
}
