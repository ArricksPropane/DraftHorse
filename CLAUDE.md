# CLAUDE.md — Arrick's Propane fork of go-mapi

Read this before changing anything. It exists so the reasoning behind this fork
survives whoever set it up.

Upstream's own project/stack reference is preserved at
`docs/UPSTREAM-CLAUDE.md`. Per-change rationale is in `PATCHES.md`.

## What this is

A fork of `marcfargas/go-mapi`, a Windows Simple MAPI → Gmail bridge. It
registers a DLL as the system default mail client so "Send to → Mail recipient"
in any Windows application creates a Gmail draft with the attachments in place.

Fork additions on top of that: created drafts open in the browser
(ARRICKS-08, `src/app/draftlink.go`, tray-toggleable), and the app registers
as a `mailto:` handler through the Windows Default Apps model (ARRICKS-09,
`DraftHorse.mailto` ProgID + Capabilities/RegisteredApplications in the
installer). A mailto click runs `DraftHorse.exe --mailto "%1"`, which opens
Gmail web compose prefilled from the URL and exits — no draft API call, no
auth, still nothing ever sent. Fleet-wide default via GPO (no Intune):
`docs/mailto-default-associations.xml`.

Drafts open in a dedicated isolated Edge/Chrome profile (ARRICKS-21,
`src/app/draftbrowser.go`, `%LOCALAPPDATA%\DraftHorse\browser-profile`, tray
toggle, default on) — the fleet uses delegated per-location mailboxes, so the
user's own browser session is the wrong account by design; IT signs the
dedicated profile into the location account once. Falls back to the default
browser when no Edge/Chrome exists. Do NOT bundle a browser (size, patching,
Google blocks sign-in in embedded WebViews). **ARRICKS-28:** `--new-window` is
deliberately NOT passed — Chromium's profile singleton then reuses the existing
window (new tab, window raised) instead of stacking one window per scan. Tabs
accumulate inside that window; closing them would need a CDP debug port on the
profile holding the location mailbox's cookies, which was considered and
rejected.

Toast XML is validated before it reaches XmlLite (ARRICKS-30,
`src/app/toast_shim_windows.go`). **Every arrival toast in every log we have
(back to 3.0.1-arricks.1) died at `LoadXml HRESULT 0xc00ce50d`** — so the
manual-mode "Create draft" button has never appeared on any machine. What
isolates it: the arrival toast is the only one carrying a raw ampersand
(`launch="action=open&emailId=..."`), and the success/error/summary toasts use
a bare `action=open` and get all the way to `put_Tag`. Apparent intermittency
in the logs is `emitArrivalToast`'s early return when the window is visible or
the app is paused — those toasts were never attempted. The builder now renders,
checks the result parses, and re-renders with escaped fields only if it does
not. CI's first run of the new tests pinned down the template's behavior
empirically: it escapes TEXT NODES (Title, Body) itself but leaves ATTRIBUTE
values raw — so the escape pass touches only the attribute-bound fields
(launch/arguments/icon); escaping Title or Body double-escapes into a literal
`&amp;` on screen.

Two Gmail accounts (V4 Phase 2, docs/V4-PLAN.md): both stay signed in; ONE is
active and every scan drafts to it. The chooser is radio rows in the tray and
a switcher in the window — never a prompt in the scan flow (Dave's explicit
model). Per-slot state: credential entries (`oauth-tokens` / `oauth-tokens-2`
under the `DraftHorse` service), signature caches + ARRICKS-29 scope flags,
and — critically — dedicated browser profiles (`browser-profile`,
`browser-profile-2`): the `/u/0` drafts URL is only correct because a profile
holds exactly ONE Google session; never let two accounts share a profile.
Slot 0 keeps the pre-V4 paths so migration and IT's signed-in profile carry
over untouched. The active manager is pinned per Gmail call; a mid-drain
switch is a documented benign race (see the accounts section in auth.go).

A default-mail guard (ARRICKS-13, `src/app/defaultmail.go`) checks at startup
and hourly that the Simple MAPI default still resolves to this app, and
self-heals a stolen default through an unelevated HKCU mirror (the stub reads
HKCU\Software\Clients\Mail before HKLM — proven end-to-end by installer-smoke
test 30). The mailto default CANNOT be set programmatically (hash-protected
UserChoice + the UCPD driver — deliberate Windows policy, do not fight it);
the app only detects it and deep-links the user to Settings > Default apps.

**Branding (ARRICKS-11, ended by V4):** the app is **DraftHorse** everywhere
— it only ever creates drafts; nothing sends. Through 3.x, ARRICKS-11 split
display (DraftHorse) from identifiers (go-mapi) for upgrade continuity; **4.0
deliberately ended that split** while the installed base was 4 test machines
(docs/V4-PLAN.md Phase 1) — the last moment the rename was cheap. Identifiers
now: `Clients\Mail\DraftHorse` key + resolver value, `DraftHorse.exe` /
`DraftHorse.dll`, AUMID `com.arrickspropane.drafthorse`, `DraftHorse.mailto`
ProgID, credential target `DraftHorse`, `%LOCALAPPDATA%\DraftHorse` +
`%APPDATA%\DraftHorse`, firewall rule, uninstall key, RMM detection.

Three things deliberately did NOT rename — do not "finish the job":
the **toastActivatorGUID** (identity, not branding — changing it recreates
the dual-registration bug it prevents), the **Go module path**
`github.com/marcfargas/go-mapi` and npm workspace names (invisible to users;
renaming breaks upstream cherry-picks and npm lockfile sync), and **source
filenames** like `go-mapi.nsi` (repo-internal). The legacy-cleanup strings in
the installer's V4 MIGRATION block and `un.RemoveScheduledTask` reference
go-mapi names ON PURPOSE — their job is scrubbing pre-4.0 installs; renaming
them breaks the migration. Per-user migration lives in `src/app/migrate.go`
(data dirs, credential target, stale HKCU heal mirror); machine-scope scrub
lives in the installer's V4 MIGRATION block.

The GCP OAuth consent-screen app name must match the DraftHorse copy in
`PreAuthModal.svelte`.

**Why a fork rather than upstream binaries:** it replaces Affixa, which retires
**31 January 2027**. Owning the source is the entire point — the fork is what
makes us independent of a single upstream maintainer.

**Primary use case: scanner "Scan to Email."** ScanSnap and Epson document
scanners, Google Workspace tenant `arrickspropane.com`. Currently **4 instances
deployed for testing**; the rollout target is **~40 Windows 11 PCs**, deployed
via ScreenConnect (no Intune/MDM — decided 2026-08-28).
When judging any change, ask what it does to a scan-to-email workflow first —
and remember that a defect ships to ~40 machines, but only 4 can catch it now.

## Architecture

| Component | Language | Path | Role |
|---|---|---|---|
| MAPI interceptor | C++17 | `src/interceptor/` | Loaded in-process by every MAPI-calling app. Writes email JSON + copies attachments into the queue. |
| Shared core | Go | `internal/mapi/` | Queue watcher, validation, Gmail client, MIME builder |
| Desktop app | Go + Svelte 5 (Wails/WebView2) | `src/app/` | Tray, auth, draft creation |
| Installer | NSIS | `src/installer/` | Mail-client registration, both bitnesses |

Queue: `%LOCALAPPDATA%\DraftHorse\queue\`. Tokens: Windows Credential Manager
(`DraftHorse` / `oauth-tokens`). **Nothing is ever sent** — the only Gmail endpoint
called is `POST /gmail/v1/users/me/drafts`.

## House rules

These encode decisions that cost real analysis. Do not undo them casually.

1. **Never reintroduce the silent auto-updater.** Upstream ships a Scheduled
   Task running as SYSTEM that downloads binaries and swaps them over
   `%ProgramFiles%`, verified only against a SHA-256 manifest served from the
   same URL as the binaries — that detects corruption, not a compromised
   release. We removed it (`updates_silent*.go`, NSIS `RegisterScheduledTask`,
   the `/AUTOUPDATE=` parser). `--update-check-silent` survives as an explicit
   no-op on purpose, so a task left behind by a prior upstream install cannot
   do anything. **Keep `un.RemoveScheduledTask`** in the uninstaller for the
   same reason.
2. **`gitHubOwner` stays `ArricksPropane`** (`src/app/updates.go`), updated
   from `egkrateia247` when the repo moved to the company account 2026-08-28
   — the constant must always track OUR real slug, never a redirecting old
   one (redirect squatting is the exact threat it guards). Upstream
   treats that constant as a security control; for a fork, leaving it unchanged
   means any surviving update path pulls a third party's binaries over our
   signed builds.
3. **Always build both x86 and x64 interceptor DLLs.** Scanner software is
   usually 32-bit while Office is 64-bit. Shipping one bitness is the classic
   way this breaks.
4. **Nothing touching the filesystem, registry, or shell APIs goes in
   `DllMain`.** It runs under the loader lock and this DLL loads into
   `explorer.exe`. Queue directories are created lazily at write time.
5. **Attachment basenames are always sanitized** via
   `message_converter::SanitizeFilename` before hitting the filesystem. Not
   theoretical: a colon in a scanner filename previously failed `CopyFileW` and
   discarded the *entire email*.
6. **Least privilege on OAuth scopes.** `gmail.compose` + userinfo, plus —
   amended by ARRICKS-24 (Dave's explicit call) — `gmail.settings.basic`,
   used for exactly one API call: reading the default sendAs signature so
   scan drafts carry it (Gmail never inserts signatures into API-created
   drafts). It is the narrowest scope that can read sendAs (no readonly
   variant exists); it can manage Gmail settings but cannot read mail. We
   dropped `gmail.send` because nothing sends. Do not add further scopes
   without the same explicit sign-off, and keep the GCP consent screen set
   to **Internal** user type — that is what bounds exposure of the client
   secret baked into the binary. The GCP OAuth client's scope list must
   include settings.basic or consent fails.

   **Adding a scope does not widen existing grants.** Refreshing a token
   returns the scopes the grant was originally issued with, so a machine that
   signed in on an older build keeps getting 403s forever, no matter how many
   times it updates. Only a fresh authorization (sign out, sign in) fixes it.
   This cost a day of silently unsigned drafts on a scanner PC in August 2026.
   ARRICKS-29 makes it self-reporting: a 403 on settings/sendAs now raises a
   tray row and one toast telling the user to sign out and back in, and the
   signature is fetched at startup so the warning appears before the first
   bad draft rather than after. **If you ever add another scope, the rollout
   step is a sign-out/in on every already-deployed machine.**
7. **Never log email bodies, subjects, or recipient addresses.** `logging.go`
   states this contract; upstream honors it and so do we.

## Build

Requires Windows. The toolchain is pinned and fussy — `build.ps1` hardcodes the
triple-prefixed clang path, so other MinGW distributions will not work.

```powershell
# one-time
scoop install mingw-mstorsjo-llvm-ucrt   # bundles cmake + ninja
scoop bucket add extras; scoop install nsis
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

# build
npm ci
npm run build:interceptor        # x64 AND x86 — both must succeed
cd src/app
wails build -platform windows/amd64 -ldflags "-X main.Version=$VERSION -X main.oauthClientID=$env:GOMAPI_OAUTH_CLIENT_ID -X main.oauthClientSecret=$env:GOMAPI_OAUTH_CLIENT_SECRET"
```

OAuth credentials live in `.env.local` (gitignored) for dev and are injected via
ldflags for release. They are never committed.

**Always verify the x86 export table before shipping.** On 32-bit,
`STDAPICALLTYPE` is `__stdcall`, so the real symbol is `_MAPISendMail@20` while
the `.def` lists the undecorated name — a known MinGW/lld footgun. A silently
broken x86 export table breaks 32-bit scanner software while everything else
looks fine:

```powershell
llvm-nm --extern-only src\interceptor\build-x86\bin\DraftHorse.dll | Select-String MAPI
```

## Test

```powershell
npm run test              # Go + Vitest
npm run check             # go vet + svelte-check
npm run test:interceptor  # DLL harness — includes the MAPISendDocuments case
go test -race ./internal/mapi/... ./src/app/...
```

The C++ unit tests link the same translation unit as the DLL, so they are
meaningful. The DLL harness was repaired in this fork (it had been watching the
wrong directory since upstream moved the queue) and is still not wired into
CTest — run it explicitly.

## State of the fork

Six commits on top of upstream `b90fcb0` (v3.0.0):

1. `DllMain` loader-lock fix
2. Attachment basename sanitization (+ the full-path fallback bug)
3. `MAPISendDocuments` implemented — was a stub returning success while
   silently discarding mail; *the* scanner-critical fix
4. DLL test harness repaired, `MAPISendDocuments` covered
5. Silent auto-updater removed, origin repointed
6. `gmail.send` scope dropped

### Known-unverified

Written and reviewed but **never compiled** — the work was done on Linux with
no Go module proxy. Treat the first successful Windows build as the real check:

- C++ touching Windows APIs: `FsUtils::RemoveAttachmentsDirForStem`
  (`FindFirstFileW` enumeration), the `test_utils.cpp` `SHGetFolderPathW`
  change, `test_send_documents.cpp`
- The Go build in its entirety
- `installer.Tests.ps1` (brace balance checked only)

Verified: pure C++ logic under `clang++ -Wall -Wextra`, 30 passing assertions;
`makensis` compiles the patched installer clean; `gofmt -e` clean; no dangling
references to deleted symbols.

### R-series hardening (ARRICKS-12) — done

The follow-up items originally listed here landed as one series:

- Attachment paths are constrained to the queue dir at watcher load
  (`pathWithinDir`, `internal/mapi/watcher.go`) — a tampered JSON can no
  longer point a draft at an arbitrary local file
- Control characters in recipient addresses are rejected at MIME build
  (CRLF header injection; error text never echoes the address per rule 7)
- Startup sweep for orphaned stem directories (15-minute age floor — the
  DLL copies attachments BEFORE writing JSON, so young dirs may be a write
  in flight)
- `nRecipCount`/`nFileCount` capped before element access; SEH
  `__try/__except` around conversion where the toolchain defines `__SEH__`
- `MaxFileSize` 18MB + cumulative `MaxTotalAttachmentSize` (base64 inflation
  math is at the constants in `gmail.go`)
- Auto mode caps consecutive permanent failures at 5, then backlog-skips the
  row; refused-connection/DNS errors now classify as "network" and never
  count toward the cap
- `GOMAPI_DEBUG_BROWSER_ARGS` only exists under `-tags gomapi_debug_browser`
- ldflags OAuth credentials win over environment variables

## Deployment

All-users install, `DraftHorse-setup.exe /S`, deployed via ScreenConnect
(per-machine runbook in ENTERPRISE.md; releases are self-signed — see
scripts/signing/ and the SELFSIGN_* secrets in installer-release.yml;
fleet machines trust the cert via trust-signing-cert.ps1). Detection rule:
`HKLM\SOFTWARE\Clients\Mail\DraftHorse` → `DLLPath`. Sign the binaries first, then
the installer that wraps them, always timestamped (`signtool /tr`).

Firewall, if egress is filtered: `accounts.google.com`, `oauth2.googleapis.com`,
`www.googleapis.com`. `ENTERPRISE.md` now carries the correct allowlist (the
upstream version listed `gmail.googleapis.com`, which broke sign-in — the
fork rewrote that doc; keep the two lists in sync).

Archive the signed installer, its SHA-256, the commit hash, and the toolchain
versions with every tagged release. That archive is the independence.
