//go:build windows

package main

// V4 migration tests — the directory logic only; keyring and registry sides
// are exercised implicitly by the installer smoke suite's migration block.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateDirFastPathRename(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "go-mapi")
	newDir := filepath.Join(root, "DraftHorse")
	if err := os.MkdirAll(filepath.Join(oldDir, "queue"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "queue", "a.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	notes := migrateDir(oldDir, newDir)
	if len(notes) != 1 {
		t.Fatalf("notes = %v, want one move note", notes)
	}
	if _, err := os.Stat(filepath.Join(newDir, "queue", "a.json")); err != nil {
		t.Errorf("queue content did not move: %v", err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Error("old dir should be gone after the fast-path rename")
	}
}

func TestMigrateDirNoopWhenNoLegacyDir(t *testing.T) {
	root := t.TempDir()
	if notes := migrateDir(filepath.Join(root, "go-mapi"), filepath.Join(root, "DraftHorse")); notes != nil {
		t.Errorf("expected silent no-op on fresh machine, got %v", notes)
	}
}

func TestMigrateDirMergeNeverClobbers(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "go-mapi")
	newDir := filepath.Join(root, "DraftHorse")
	for d, content := range map[string]string{
		filepath.Join(oldDir, "settings.json"): "old",
		filepath.Join(newDir, "settings.json"): "new",
		filepath.Join(oldDir, "app.log"):       "old-log",
	} {
		if err := os.MkdirAll(filepath.Dir(d), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(d, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	migrateDir(oldDir, newDir)

	// The new side's file must win — a fresh component wrote it after 4.0
	// started and the stale legacy copy must not overwrite it.
	got, err := os.ReadFile(filepath.Join(newDir, "settings.json"))
	if err != nil || string(got) != "new" {
		t.Errorf("settings.json = %q, %v; want untouched \"new\"", got, err)
	}
	// The file only the old side had must arrive.
	if got, err := os.ReadFile(filepath.Join(newDir, "app.log")); err != nil || string(got) != "old-log" {
		t.Errorf("app.log = %q, %v; want migrated \"old-log\"", got, err)
	}
	// Old dir still holds the skipped conflict — never silently deleted.
	if _, err := os.Stat(filepath.Join(oldDir, "settings.json")); err != nil {
		t.Error("conflicting legacy file must remain in place for manual inspection")
	}
}
