package main

// AUMIDs — pinned per RESEARCH §2 + CONTEXT.md §Specifics.
const (
	aumidDev  = "com.arrickspropane.drafthorse.dev"
	aumidProd = "com.arrickspropane.drafthorse" // Phase 10 installer uses this; kept here for source truth.
)

// toastActivatorGUID is the DraftHorse-owned COM CLSID for toast activation.
// Generated fresh for Phase 9 (RESEARCH §9 landmines 2 + 8 — NEVER use the
// jackmordaunt default {0F82E845-...} default GUID). MUST be identical across
// dev + prod builds so a user upgrading from dev to prod does not end up with
// two competing HKCU registrations.
//
// Generated: 2026-04-19 via PowerShell [guid]::NewGuid(); pinned here.
const toastActivatorGUID = "{6352C677-78F0-444F-AAA9-724EB43DBCB0}"

// toastGroup is the shared group string for all Phase 9 toasts. Enables
// Windows to auto-collapse multiple pending toasts under a single DraftHorse
// banner in Action Center (D-08).
const toastGroup = "DraftHorse-queue"

// Toast copy constants — matches UI-SPEC §Copywriting Contract exactly.
// Kept here as a single source of truth so copy changes are localized.
const (
	toastCopyDraftFailedSignedOut = "Draft failed — Sign-in expired"
	toastCopyDraftFailedNetwork   = "Draft failed — Network error"
	toastCopyDraftFailedGmail     = "Draft failed — Gmail error"
	toastCopySummaryInvalidGrant  = "Sign-in expired — emails queued for manual review"

	// ARRICKS-29. Deliberately states the remedy, not the cause: "your
	// sign-in predates the signature permission" means nothing to the person
	// at the scanner. Sign out, sign in — that is the whole fix.
	toastCopySignatureScopeTitle = "DraftHorse: signature missing from drafts"
	toastCopySignatureScopeBody  = "Sign out and back in once to restore your Gmail signature."
)

// toastErrorCopy maps a Plan 03 errorCategory to user-facing error text.
func toastErrorCopy(category string) string {
	switch category {
	case "signed-out":
		return toastCopyDraftFailedSignedOut
	case "network":
		return toastCopyDraftFailedNetwork
	case "gmail":
		return toastCopyDraftFailedGmail
	default:
		return toastCopyDraftFailedGmail // fallback — should never hit in practice
	}
}

// aumidOverride is injected at build time via:
//
//	-ldflags "-X 'main.aumidOverride=com.arrickspropane.drafthorse'"
//
// `var` (not const) is REQUIRED — -X only overwrites string vars. Mirrors the
// pattern used for oauthClientID / oauthClientSecret in auth_credentials.go.
// Phase 10 installer wires this to aumidProd in its release build flags.
var aumidOverride string

// activeAUMID picks the correct AUMID for this build.
// Returns the ldflags-injected value when set (release builds); falls back to
// aumidDev so that dev/test runs continue to work without any -X flag.
func activeAUMID() string {
	if aumidOverride != "" {
		return aumidOverride
	}
	return aumidDev
}
