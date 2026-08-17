package mapi

// ARRICKS-12 R-series hardening coverage:
//   R3 — attachment paths constrained to the queue dir (watcher)
//   R4 — control characters in recipient addresses rejected (BuildFullMIME)
//   R5 — orphaned stem directories swept at watcher start
//   R9 — per-file and cumulative attachment size caps (BuildFullMIME)

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- R4: header address injection ---

func TestBuildFullMIME_RejectsCRLFInAddress(t *testing.T) {
	msg := &MailMessage{
		BodyFormat: "plain",
		Body:       "body",
		Subject:    "subject",
		Recipients: Recipients{
			To: []Recipient{{Address: "victim@example.com\r\nBcc: attacker@evil.example"}},
		},
	}
	_, err := BuildFullMIME(msg)
	if err == nil {
		t.Fatal("expected error for CRLF in recipient address, got nil")
	}
	if !strings.Contains(err.Error(), "control characters") {
		t.Errorf("unexpected error: %v", err)
	}
	// House rule 7: the error text must never contain the address itself.
	if strings.Contains(err.Error(), "victim@") || strings.Contains(err.Error(), "evil.example") {
		t.Errorf("error text leaks the recipient address: %v", err)
	}
}

func TestBuildFullMIME_RejectsControlCharInCCAndBCC(t *testing.T) {
	for _, tc := range []struct {
		name string
		rec  Recipients
	}{
		{"cc", Recipients{
			To: []Recipient{{Address: "ok@example.com"}},
			CC: []Recipient{{Address: "bad\x00@example.com"}},
		}},
		{"bcc", Recipients{
			To:  []Recipient{{Address: "ok@example.com"}},
			BCC: []Recipient{{Address: "bad\x7f@example.com"}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &MailMessage{BodyFormat: "plain", Body: "b", Subject: "s", Recipients: tc.rec}
			if _, err := BuildFullMIME(msg); err == nil {
				t.Fatalf("expected error for control char in %s address", tc.name)
			}
		})
	}
}

func TestBuildFullMIME_PlainAddressStillAccepted(t *testing.T) {
	msg := &MailMessage{
		BodyFormat: "plain",
		Body:       "body",
		Subject:    "subject",
		Recipients: Recipients{
			To: []Recipient{{Name: "Óffice Scanner", Address: "scans@arrickspropane.com"}},
		},
	}
	out, err := BuildFullMIME(msg)
	if err != nil {
		t.Fatalf("BuildFullMIME: %v", err)
	}
	if !strings.Contains(string(out), "scans@arrickspropane.com") {
		t.Error("expected recipient address in output")
	}
}

// --- R9: size caps ---

// makeSizedFile creates a file whose Stat size is n bytes without writing n
// bytes of data (sparse via Truncate).
func makeSizedFile(t *testing.T, dir, name string, n int64) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create %s: %v", p, err)
	}
	if err := f.Truncate(n); err != nil {
		f.Close()
		t.Fatalf("truncate %s: %v", p, err)
	}
	f.Close()
	return p
}

func TestBuildFullMIME_PerFileSizeCap(t *testing.T) {
	dir := t.TempDir()
	big := makeSizedFile(t, dir, "big.pdf", MaxFileSize+1)
	msg := &MailMessage{
		BodyFormat:  "plain",
		Body:        "b",
		Subject:     "s",
		Recipients:  Recipients{To: []Recipient{{Address: "ok@example.com"}}},
		Attachments: []Attachment{{Filename: "big.pdf", Path: big}},
	}
	_, err := BuildFullMIME(msg)
	if err == nil || !strings.Contains(err.Error(), "attachment too large") {
		t.Fatalf("expected per-file size error, got %v", err)
	}
}

func TestBuildFullMIME_TotalSizeCap(t *testing.T) {
	dir := t.TempDir()
	// Each file passes the per-file cap; together they exceed the total cap.
	half := int64(MaxTotalAttachmentSize)/2 + 1024
	a := makeSizedFile(t, dir, "page1.pdf", half)
	b := makeSizedFile(t, dir, "page2.pdf", half)
	msg := &MailMessage{
		BodyFormat: "plain",
		Body:       "b",
		Subject:    "s",
		Recipients: Recipients{To: []Recipient{{Address: "ok@example.com"}}},
		Attachments: []Attachment{
			{Filename: "page1.pdf", Path: a},
			{Filename: "page2.pdf", Path: b},
		},
	}
	_, err := BuildFullMIME(msg)
	if err == nil || !strings.Contains(err.Error(), "attachments too large in total") {
		t.Fatalf("expected total-size error, got %v", err)
	}
}

// --- R3: attachment path containment ---

func TestPathWithinDir(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		p    string
		want bool
	}{
		{filepath.Join(root, "stem", "scan.pdf"), true},
		{filepath.Join(root, "direct.pdf"), true},
		{root, true}, // the root itself resolves to rel "."
		{filepath.Join(root, "..", "outside.pdf"), false},
		{filepath.Join(root, "stem", "..", "..", "outside.pdf"), false},
		{filepath.Dir(root), false},
	}
	for _, c := range cases {
		if got := pathWithinDir(root, c.p); got != c.want {
			t.Errorf("pathWithinDir(%q, %q) = %v, want %v", root, c.p, got, c.want)
		}
	}
}

// recordingCallback records dispatched errors for assertions.
type recordingCallback struct {
	errs chan error
}

func (r *recordingCallback) OnQueueChanged(_ []EmailWithId) {}
func (r *recordingCallback) OnError(err error)              { r.errs <- err }

func TestWatcher_AttachmentPathEscapeGoesToErrors(t *testing.T) {
	watchDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "loot.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	cb := &recordingCallback{errs: make(chan error, 4)}
	ew, err := NewEmailWatcher(watchDir, cb)
	if err != nil {
		t.Fatal(err)
	}
	defer ew.Stop()

	msg := MailMessage{
		Version:    1,
		Timestamp:  "2024-01-01T00:00:00Z",
		BodyFormat: "plain",
		Recipients: Recipients{To: []Recipient{{Address: "to@example.com"}}},
		Attachments: []Attachment{
			{Filename: "loot.txt", Path: outside},
		},
	}
	data, _ := json.Marshal(msg)
	if err := os.WriteFile(filepath.Join(watchDir, "escape.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ew.Start(); err != nil {
		t.Fatal(err)
	}

	select {
	case werr := <-cb.errs:
		if !strings.Contains(werr.Error(), "escapes queue directory") {
			t.Errorf("unexpected watcher error: %v", werr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("expected a dispatched error for escaping attachment path")
	}
	if n := len(ew.Snapshot()); n != 0 {
		t.Errorf("expected empty queue, got %d entries", n)
	}
	if _, err := os.Stat(filepath.Join(watchDir, "errors", "escape.json")); err != nil {
		t.Errorf("expected escape.json moved to errors dir: %v", err)
	}
}

func TestWatcher_AttachmentPathInsideQueueAccepted(t *testing.T) {
	watchDir := t.TempDir()
	stemDir := filepath.Join(watchDir, "ok-stem")
	if err := os.MkdirAll(stemDir, 0755); err != nil {
		t.Fatal(err)
	}
	att := filepath.Join(stemDir, "scan.pdf")
	if err := os.WriteFile(att, []byte("pdf"), 0644); err != nil {
		t.Fatal(err)
	}

	ew, err := NewEmailWatcher(watchDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ew.Stop()

	msg := MailMessage{
		Version:     1,
		Timestamp:   "2024-01-01T00:00:00Z",
		BodyFormat:  "plain",
		Recipients:  Recipients{To: []Recipient{{Address: "to@example.com"}}},
		Attachments: []Attachment{{Filename: "scan.pdf", Path: att}},
	}
	data, _ := json.Marshal(msg)
	if err := os.WriteFile(filepath.Join(watchDir, "ok-stem.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ew.Start(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(ew.Snapshot()) == 1 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("expected in-queue attachment path to be accepted")
}

// --- R5: orphan sweep ---

func TestSweepOrphanedStemDirs(t *testing.T) {
	watchDir := t.TempDir()
	old := time.Now().Add(-2 * time.Hour)

	mk := func(name string) string {
		p := filepath.Join(watchDir, name)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		return p
	}

	orphanOld := mk("orphan-old")
	if err := os.WriteFile(filepath.Join(orphanOld, "leak.pdf"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(orphanOld, old, old); err != nil {
		t.Fatal(err)
	}

	orphanFresh := mk("orphan-fresh") // young: may be a DLL write in flight

	live := mk("live-stem") // has a matching JSON — never an orphan
	if err := os.Chtimes(live, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(watchDir, "live-stem.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	ew, err := NewEmailWatcher(watchDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ew.Stop()
	// processed/ and errors/ were just created by NewEmailWatcher (young),
	// but pin them old to prove the name filter alone protects them.
	for _, d := range []string{"processed", "errors"} {
		if err := os.Chtimes(filepath.Join(watchDir, d), old, old); err != nil {
			t.Fatal(err)
		}
	}

	ew.sweepOrphanedStemDirs()

	if _, err := os.Stat(orphanOld); !os.IsNotExist(err) {
		t.Error("old orphan dir should have been removed")
	}
	for name, p := range map[string]string{
		"fresh orphan": orphanFresh,
		"live stem":    live,
		"processed":    filepath.Join(watchDir, "processed"),
		"errors":       filepath.Join(watchDir, "errors"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s dir should have survived the sweep: %v", name, err)
		}
	}
}
