//go:build windows

package main

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

// --- pure decision logic ---

func TestMailClientNeedsRepair(t *testing.T) {
	cases := []struct {
		name         string
		effective    string
		hkcuPtr      string
		mirrorIntact bool
		want         bool
	}{
		{"healthy via HKLM", "go-mapi", "", false, false},
		{"healthy via HKCU mirror", "go-mapi", "go-mapi", true, false},
		{"stolen by outlook", "Microsoft Outlook", "", false, true},
		{"stolen via HKCU override", "Microsoft Outlook", "Microsoft Outlook", false, true},
		{"empty (no default at all)", "", "", false, true},
		{"HKCU points at us but mirror broken", "go-mapi", "go-mapi", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mailClientNeedsRepair(c.effective, c.hkcuPtr, c.mirrorIntact); got != c.want {
				t.Errorf("mailClientNeedsRepair(%q, %q, %v) = %v, want %v",
					c.effective, c.hkcuPtr, c.mirrorIntact, got, c.want)
			}
		})
	}
}

// --- repair write shape, against a real (sandboxed) HKCU subtree ---

// TestRepairMailClientDefault_WriteShape redirects the guard's key path into
// a throwaway HKCU subtree, runs the repair, and asserts the exact registry
// shape the self-heal must produce: a self-consistent mirror (display name +
// REG_EXPAND_SZ DLLPath) plus the (Default) pointer. Never touches the real
// Software\Clients\Mail.
func TestRepairMailClientDefault_WriteShape(t *testing.T) {
	const sandbox = `Software\go-mapi-test\ARRICKS13\Clients\Mail`
	orig := mapiClientsMailPath
	mapiClientsMailPath = sandbox
	t.Cleanup(func() {
		mapiClientsMailPath = orig
		_ = registry.DeleteKey(registry.CURRENT_USER, sandbox+`\go-mapi`)
		_ = registry.DeleteKey(registry.CURRENT_USER, sandbox)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\go-mapi-test\ARRICKS13\Clients`)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\go-mapi-test\ARRICKS13`)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\go-mapi-test`)
	})

	if err := repairMailClientDefault(); err != nil {
		t.Fatalf("repairMailClientDefault: %v", err)
	}

	// Pointer.
	parent, err := registry.OpenKey(registry.CURRENT_USER, sandbox, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("open sandbox parent: %v", err)
	}
	defer parent.Close()
	ptr, _, err := parent.GetStringValue("")
	if err != nil || ptr != "go-mapi" {
		t.Errorf("(Default) pointer = %q, %v; want go-mapi", ptr, err)
	}

	// Mirror subkey: display name + expandable DLLPath.
	sub, err := registry.OpenKey(registry.CURRENT_USER, sandbox+`\go-mapi`, registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("open mirror subkey: %v", err)
	}
	defer sub.Close()
	disp, _, err := sub.GetStringValue("")
	if err != nil || disp != "DraftHorse" {
		t.Errorf("mirror (Default) = %q, %v; want DraftHorse", disp, err)
	}
	dll, valType, err := sub.GetStringValue("DLLPath")
	if err != nil {
		t.Fatalf("read DLLPath: %v", err)
	}
	if valType != registry.EXPAND_SZ {
		t.Errorf("DLLPath value type = %d, want EXPAND_SZ (%d) — a plain SZ would break per-bitness expansion (ARRICKS-10)", valType, registry.EXPAND_SZ)
	}
	if dll != `%ProgramFiles%\go-mapi\go-mapi.dll` {
		t.Errorf("DLLPath = %q, want unexpanded %%ProgramFiles%% form", dll)
	}

	// Round-trip: with the sandbox populated, the effective reader sees us
	// and the needs-repair decision goes quiet.
	if got := readMapiEffectiveClient(); got != "go-mapi" {
		t.Errorf("readMapiEffectiveClient after repair = %q, want go-mapi", got)
	}
	if mailClientNeedsRepair(readMapiEffectiveClient(), readHKCUMailPointer(), hkcuMirrorIntact()) {
		t.Error("mailClientNeedsRepair should be false immediately after repair")
	}
}
