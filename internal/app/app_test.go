package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveJournalFileUsesExplicitPath(t *testing.T) {
	journal := writeJournal(t, filepath.Join(t.TempDir(), "main.journal"))
	t.Setenv("LEDGER_FILE", "")

	assertResolvedJournal(t, "", journal, journal)
}

func TestResolveJournalFileUsesLedgerFileEnvironment(t *testing.T) {
	journal := writeJournal(t, filepath.Join(t.TempDir(), "main.journal"))
	t.Setenv("LEDGER_FILE", journal)

	assertResolvedJournal(t, "", "", journal)
}

func TestResolveJournalFileExpandsHomeInLedgerFile(t *testing.T) {
	home := t.TempDir()
	journal := writeJournal(t, filepath.Join(home, "finance", "main.journal"))
	t.Setenv("HOME", home)
	t.Setenv("LEDGER_FILE", "~/finance/main.journal")

	assertResolvedJournal(t, "", "", journal)
}

func TestResolveJournalFileUsesHomeDefault(t *testing.T) {
	home := t.TempDir()
	journal := writeJournal(t, filepath.Join(home, ".hledger.journal"))
	t.Setenv("HOME", home)
	t.Setenv("LEDGER_FILE", "")

	assertResolvedJournal(t, "", "", journal)
}

func TestResolveJournalFileIgnoresWorkingDirectory(t *testing.T) {
	workdir := t.TempDir()
	writeJournal(t, filepath.Join(workdir, ".hledger.journal"))
	writeJournal(t, filepath.Join(workdir, "finance", "journal", ".hledger.journal"))
	t.Chdir(workdir)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LEDGER_FILE", "")

	if got, err := resolveJournalFile(""); err == nil {
		t.Fatalf("resolveJournalFile() = %q, want error: journals in the working directory must not be used", got)
	}
}

func assertResolvedJournal(t *testing.T, _ string, explicit, want string) {
	t.Helper()
	got, err := resolveJournalFile(explicit)
	if err != nil {
		t.Fatalf("resolveJournalFile(%q) error = %v", explicit, err)
	}
	if got != want {
		t.Fatalf("resolveJournalFile(%q) = %q, want %q", explicit, got, want)
	}
}

func writeJournal(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
