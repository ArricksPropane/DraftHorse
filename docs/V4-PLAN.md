# DraftHorse 4.0 plan — rename, two accounts, ScanSnap

Status: DRAFT for Dave's review. No code has been written against this plan.
Decided 2026-08-26: rename scope = all visible identifiers; account model =
active-account toggle (tray + window); build order = plan first.

4.0 is the wider-release vehicle: all three changes land BEFORE the ~40-machine
Intune rollout, while the installed base is 4 test machines. That timing is the
entire justification for Phase 1 — see below.

---

## Phase 1 — Rename: go-mapi → DraftHorse (breaking; the reason this is 4.0)

This deliberately reverses the ARRICKS-11 branding split ("display DraftHorse,
identifiers go-mapi"). The split existed for upgrade continuity on an installed
fleet. The fleet is 4 test machines that IT must touch anyway (scope re-auth),
and after the Intune rollout the price of this rename becomes a 40-machine
migration, permanently. It is now or never.

### Renames

| Identifier | Old | New |
|---|---|---|
| MAPI client key | `HKLM\SOFTWARE\Clients\Mail\go-mapi` | `...\Clients\Mail\DraftHorse` |
| Resolver value | `Clients\Mail (Default) = "go-mapi"` | `= "DraftHorse"` |
| HKCU heal mirror (ARRICKS-13) | `HKCU\Software\Clients\Mail\go-mapi` | `...\DraftHorse` |
| App binary | `go-mapi.exe` | `DraftHorse.exe` |
| Interceptor DLLs | `go-mapi.dll` (x86+x64) | `DraftHorse.dll` (x86+x64) |
| AUMID | `com.marcfargas.gomapi` | `com.arrickspropane.drafthorse` |
| mailto ProgID | `go-mapi.mailto` | `DraftHorse.mailto` |
| Credential target | `go-mapi` / `oauth-tokens` | `DraftHorse` / `oauth-tokens` |
| Data dirs | `%LOCALAPPDATA%\go-mapi`, `%APPDATA%\go-mapi` | `...\DraftHorse` |
| Autostart | `Run\go-mapi` | `Run\DraftHorse` |
| Firewall rule / uninstall key / Intune detection | go-mapi names | DraftHorse names |
| Settings deep link | `?registeredAppMachine=DraftHorse` | unchanged (already display) |

### Deliberately NOT renamed

- **toastActivatorGUID** — identity, not branding. Changing it recreates the
  dual-registration bug it exists to prevent.
- **Go module path** `github.com/marcfargas/go-mapi` + package names —
  invisible to users; renaming would make upstream cherry-picks manual forever.
- Repo name/URL — already DraftHorse.

### Migration (runs once, 4 machines)

1. **Installer**: after installing under new names, move
   `%LOCALAPPDATA%\go-mapi` → `%LOCALAPPDATA%\DraftHorse` (queue AND
   browser-profile — the move preserves the IT-signed Edge session's cookies)
   and `%APPDATA%\go-mapi` → `%APPDATA%\DraftHorse` (settings + log). Risk:
   profile locked if the dedicated Edge is running — installer closes it or
   falls back to copy-then-delete-on-reboot. Then delete old registry keys:
   `Clients\Mail\go-mapi` (HKLM + HKCU mirror), `Run\go-mapi`,
   `go-mapi.mailto` ProgID + RegisteredApplications entry, firewall rule,
   old uninstall key.
2. **App first run**: read tokens from old credential target `go-mapi`; if
   found and new target empty, write to `DraftHorse`, delete old. Same user
   context, so silent. Any failure = signed-out state, and the sign-in flow
   already exists (pre-3.8 machines re-auth anyway).
3. **Intune**: detection rule and mailto XML updated to new names BEFORE the
   rollout is created — nothing to migrate fleet-side because nothing is
   deployed yet.

### Test impact (the real cost)

ARRICKS-10/13/20 registry behavior was CI-proven under old names and must be
re-proven under new ones: all 35 installer smoke tests' hardcoded strings,
mapiprobe tests 28/29 (both bitnesses), test 30 (HKCU-outranks-HKLM),
collect-registration.ps1 (bump to v2.0), mailto-default-associations.xml,
ENTERPRISE.md, CLAUDE.md branding section (rule rewritten, not deleted — it
must record WHY the split existed and why 4.0 ended it). Plus new migration
smoke tests: install 3.9 → upgrade to 4.0 → assert queue/settings/tokens
survive and old keys are gone.

**Phase 1 ships alone as `v4.0.0-arricks.1` before anything else lands.**
A rename bug and a feature bug in the same build are indistinguishable.

---

## Phase 2 — Two Gmail accounts, active-account toggle

Model (decided): both accounts stay signed in; one is ACTIVE. Every scan
drafts to the active account. Zero prompts in the scan flow.

- **Chooser surfaces** (per Dave's question: both): tray menu gets two
  radio-style rows ("Draft to: kaylah@…" / "Draft to: scans-…@…") using the
  existing checkbox-item pattern; the main window gets the same switch plus
  "Add second account" / per-account sign-out. Set it before scanning at
  either surface; takes effect for all subsequent scans; persisted in
  settings (`active_account`).
- **Storage**: credential entries per slot (`DraftHorse` /
  `oauth-tokens-1|2`); AccountStore owns up to two AuthManagers; existing
  token migrates to slot 1.
- **Browser profiles — the invisible constraint**: one dedicated profile PER
  account (`browser-profile\1`, `browser-profile\2`). The drafts-list URL's
  `/u/0` trick (ARRICKS-22) is only correct because a profile holds exactly
  one session; two accounts in one profile silently opens the wrong mailbox.
  Existing profile migrates to slot 1. IT signs profile 2 in once, same as
  the ARRICKS-21 runbook step.
- **Per-account state**: signature cache (ARRICKS-24) and the scope-missing
  flag (ARRICKS-29) become per-slot; sign-out resets only that slot.
- **Surfaces that name the account**: tray tooltip shows the active account;
  draft-created toast unchanged (subject already identifies the scan).
- Scope stays at exactly two accounts in the UI; storage is a list so a third
  is a UI change, not a refactor.

---

## Phase 3 — ScanSnap Home "Scan to E-mail" — ROOT CAUSE CONFIRMED, FIXED

**2026-08-28, Procmon trace (Dave):** hypothesis 2 exactly. ScanSnap's
ScanToMail.exe resolves the default client name from Clients\Mail (Default)
— HKCU layer first, honoring the ARRICKS-13 mirror — then REQUIRES
`Clients\Mail\<client>\shell\open\command`, retrying it four times before
declaring "no email client installed". It never reads DLLPath. DraftHorse
wrote only DLLPath, so detection failed while Simple MAPI itself (Send To →
Mail recipient) worked — which is also why the 08-27 "pass" was a false
positive: it exercised the MAPI path, not ScanSnap's own button.

Fix: the installer writes `shell\open\command = "$INSTDIR\DraftHorse.exe"`
on the client key; the ARRICKS-13 heal mirror writes the same (resolved from
the running exe), and the mirror-intact check requires it so pre-fix mirrors
self-upgrade via the hourly guard without a reinstall. Smoke test 3 and the
diagnostics script assert it.

**Update 2026-08-28 (earlier):** the error returned on the machine that
passed the day before — the 08-27 "resolved" call below was premature.

### Superseded 08-27 note (kept for the record)

**Outcome (2026-08-27, Dave's testing on v4.0.0-arricks.3):** Scan to E-mail
works. No Procmon trace was ever run and no registry fix shipped — the Phase 1
rename + stale-state scrub incidentally cleared whatever ScanSnap's detection
was rejecting (most plausibly the dead Applications keys / dangling old
client-key DLL path, not the win.ini MAPI markers hypothesized below). Root
cause therefore UNCONFIRMED: if "no email client installed" ever returns, the
investigation protocol below is still the play. The hypotheses are kept as
written for that eventuality.

### Original plan (kept for the record)

Observed: ScanSnap Home reports "no email client installed" even though
mailto, .mapimail, and .ml associations point at DraftHorse. So its detection
reads something DraftHorse does not set. Verified 2026-08-26: the installer
writes NO "Windows Messaging Subsystem" markers and NO shell\open\command
under the Clients\Mail client key. Both classic detection points are unset.

### Step 1 — pinpoint the check (one scanner PC, ~30 min)

Process Monitor, filter Process Name = ScanSnap Home's exe, Operation =
RegQueryValue/RegOpenKey, Result = NAME NOT FOUND. Trigger Scan to E-mail,
save the trace around the error dialog. The failed reads name the exact keys.

### Hypotheses, in order of likelihood

1. **Simple MAPI availability markers**:
   `HKLM\SOFTWARE\Microsoft\Windows Messaging Subsystem` values `MAPI="1"`
   (often `MAPIX="1"`), the registry mapping of win.ini `[Mail] MAPI=1` —
   the decades-old "is a MAPI client installed?" check. Typically written by
   Outlook; absent on these PCs. Fix: installer writes them (both registry
   views; shared-with-Outlook semantics documented). Contained, low risk.
2. **`shell\open\command` missing under `Clients\Mail\<client>`** — some
   enumerators require an openable client, not just a DLLPath. Fix: add
   `shell\open\command = "DraftHorse.exe"` under the client key.
3. **Hardcoded client whitelist** (Outlook/Thunderbird/Windows Mail). If the
   trace shows name-specific probes, appearing in the picker may be
   impossible; fallback position is documented ("use Scan to Folder +
   Send To", which already works) and we stop there.

Fix ships only after the trace confirms which hypothesis holds — no
speculative registry writes (they'd be fleet-wide and shared with Office).

---

## Sequencing

1. v3.9.0-arricks.1 scanner retest (unchanged priority — validates
   ARRICKS-28/29/30 before the rename churns everything)
2. Phase 1 rename → `v4.0.0-arricks.1` → full smoke + migration retest
3. Phase 3 Procmon trace (independent; can happen any time on a scanner PC)
4. Phase 2 accounts → `v4.0.0-arricks.2`
5. Phase 3 fix (likely rides .2 or .3) → clean retest → **`v4.0.0` production
   tag → Intune rollout of the ~40 machines**

The 3.9.0 production tag is superseded: wider release ships as 4.0.

## Open questions for Dave

1. AUMID `com.arrickspropane.drafthorse` — happy with that identity, or
   prefer keeping `com.marcfargas.gomapi` lineage?
2. Second account per machine: is it always "personal + location", or could
   some machines need two location accounts? (Affects nothing structurally —
   only runbook wording.)
3. GCP consent screen: same OAuth client serves both accounts (both are
   arrickspropane.com, Internal type) — confirm no per-account client wanted.
