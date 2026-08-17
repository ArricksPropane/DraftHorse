# go-mapi hardening patches — egkrateia247/go-mapi

**Base:** upstream `marcfargas/go-mapi` @ `b90fcb0` (v3.0.0)
**Branch:** `arricks-hardening`
**Commits:** 6
**Net:** +658 / −1146 lines across 24 files

---

## Applying

```powershell
git clone https://github.com/egkrateia247/go-mapi.git
cd go-mapi
git remote add upstream https://github.com/marcfargas/go-mapi.git
git fetch upstream
git checkout -b arricks-hardening b90fcb0
git am ..\go-mapi-arricks-hardening.patch
```

`git am` preserves the six commits separately, so any one can be reverted or
reviewed on its own. If you would rather review a single diff first, use
`go-mapi-arricks-hardening.diff` with `git apply`.

Every commit message states the problem, the evidence, and the reasoning. They
are written to be read by whoever inherits this in two years.

---

## The commits

### 1. `d5fe484` — remove filesystem work from `DllMain` *(blocker B3)*

`DllMain` called `FsUtils::EnsureOutputDirectory()`, which reaches
`SHGetFolderPathW` and `SHCreateDirectoryExW`. Shell path resolution loads
further DLLs and reads the registry under the loader lock — explicitly
forbidden by Microsoft's DLL guidance — and the `std::wstring` it builds can
throw `bad_alloc` into the loader.

This DLL loads in-process into every application that calls Simple MAPI,
including `explorer.exe`. A deadlock here hangs the host at load time, before
any email is involved.

The call was also redundant: `JsonWriter::WriteMailToFile` and
`WriteMailToFileWithStem` both call `EnsureOutputDirectory()` where the
directory is actually needed. Replaced with `DisableThreadLibraryCalls`.

**Risk: very low.** Deletes a redundant call.

---

### 2. `2b31ad4` — sanitize attachment basenames *(the filename fix)*

Adds `message_converter::SanitizeFilename()` and routes the attachment basename
through it before the copy.

The likely-in-practice failure was mundane rather than exotic: any NTFS-illegal
character in `lpszFileName` made `CopyFileW` fail, which returned
`MAPI_E_FAILURE` and **discarded the entire email**. A scan named
`Scan 2026-08-14 09:12:33.pdf` triggers it, and the user sees only a generic
error from the host application. The security case — `..\..\evil.lnk` writing
outside the queue — is real but secondary.

This also fixes the related fallback bug: `message_converter` guards on the
`lpszFileName` *pointer* rather than on emptiness, so a non-NULL but empty
`lpszFileName` (common in older MFC wrappers) left `att.filename` empty and
`mapi_impl` fell back to the **full path**, producing
`...\queue\<stem>\C:\TEMP\scan.pdf`. `SanitizeFilename` reduces a path to its
leaf, so both cases collapse to the same safe answer.

Guarantees on the output: no path separators, no NTFS-illegal or control
characters, no trailing dot or space, not a reserved DOS device name, valid
UTF-8, ≤128 bytes, never empty.

`att.filename` is deliberately left untouched, so the draft's MIME filename
still shows the caller's original name. Only the on-disk copy is renamed.

Adds 11 test cases including UTF-8-safe truncation.

**Risk: low.** New pure function; the only behavioral change is that names
which previously failed the copy now succeed.

---

### 3. `8262550` — implement `MAPISendDocuments` *(blocker B1 — the scanner one)*

Was a stub returning `SUCCESS_SUCCESS` without doing anything. Callers were
told the mail had been handled, typically deleted their own temp file, and
nothing was ever queued.

Implemented properly rather than returning `MAPI_E_NOT_SUPPORTED`: split the
delimited path and display-name lists (defaulting to `;` when `lpszDelimChar`
is absent), build a recipient-less message, queue it like any other. Text is
converted to UTF-8 *before* splitting so a DBCS lead-byte pair cannot be cut in
half by an ASCII delimiter.

Also extracts `QueueMessage()` as the shared tail for all three send entry
points, and fixes a leak that refactor exposed: the attachment-copy failure
path had rollback but the JSON-write failure path had none, so a failed write
left attachment copies in `%LOCALAPPDATA%` permanently — there is no orphan
sweep anywhere in the Go app. Adds `FsUtils::RemoveAttachmentsDirForStem`.

**Risk: moderate — this is the one to test hardest.** It changes an entry point
that currently "succeeds" (by doing nothing) into one that does real work.
Section 9.2 of the runbook covers the tests.

---

### 4. `97ea45b` — repair the DLL test harness, cover `MAPISendDocuments`

The harness monitored `%TEMP%\go-mapi` while the DLL has written to
`%LOCALAPPDATA%\go-mapi\queue` since `quick/260423-msq`, so every test was
watching a directory the DLL never touches. It is not wired into CTest either,
so the breakage was invisible.

- points `GetGoMapiTempDir()` at the real queue directory
- `test_with_attachments` used a hardcoded `C:\test.txt` that exists on no
  clean machine; creates a real file now
- adds `test_send_documents`, which drives the real export with two files and a
  semicolon-delimited list and asserts a queue JSON was produced and both
  copies landed. The old stub also returned `SUCCESS_SUCCESS`, so the return
  code alone is not a useful assertion — the queue file is
- links `shell32` for `SHGetFolderPathW`

**Risk: none to production.** Test-only.

---

### 5. `8290bc4` — remove the silent auto-updater; repoint origin *(blocker B2)*

- `main.go`: silent dispatch removed. The `--update-check-silent` flag is
  **retained as an explicit no-op** so a Scheduled Task left behind by a
  previous upstream install cannot do anything if it fires against your binary.
- deletes `updates_silent.go`, `updates_silent_bindings.go`, and their test
  (445 + 11 + 442 lines) — the download-and-swap code is gone from the binary
- `updates.go`: `gitHubOwner` → `egkrateia247`, so no code path can resolve to
  upstream even if an update path is reintroduced later
- `settings.go`: update checks default **off** — you deploy through Intune, so
  the notify check has nothing useful to say and is one less outbound
  dependency on 12 machines
- `go-mapi.nsi`: removes the `/AUTOUPDATE=` parser, the opt-in page, and
  `RegisterScheduledTask`. **`un.RemoveScheduledTask` is deliberately kept** so
  uninstalling over a machine that previously had an upstream build still
  removes its task
- `installer.Tests.ps1`: test 22 inverted into a guard that the task is *never*
  created; adds 22b asserting the flag is inert

**Risk: low.** Removal only. The `/AUTOUPDATE=1` flag is now silently ignored
rather than honored, so a deployment script still carrying it cannot re-arm
anything.

---

### 6. `a4cfa9c` — drop the unused `gmail.send` scope

Requested but never used — the only Gmail endpoint this codebase calls is
`POST /gmail/v1/users/me/drafts`. The effect is on the consent screen, which
told every user the app could send mail as them when it structurally cannot.

**Risk: none.** Existing users will re-consent once on next sign-in because the
scope set changed.

---

## What I verified, and what I could not

Honesty matters more than a clean bill of health here, so:

### Verified

| Check | Result |
|---|---|
| Pure C++ logic (`SanitizeFilename`, `SplitDelimited`) compiled with `clang++ -std=c++17 -Wall -Wextra` | clean, no warnings |
| 30 behavioral assertions against that logic — illegal characters, traversal, ADS, control characters, reserved names, length cap, UTF-8-safe truncation, delimiter edge cases | **all pass** |
| `makensis` compile of the patched installer script | **succeeds** — 5 pages vs 6, 640 instructions vs 876, and the same single pre-existing warning as baseline |
| Go parse/format check (`gofmt -e`) on every changed file | clean; the two files gofmt flags were already flagged before these patches |
| No dangling references to deleted symbols (`runSilentUpdate`, `swapInPlace`, `checksumsURL`, …) | none |
| Pester test file brace balance vs baseline | balanced, +1 `It` block as expected |
| Every file:line claim in the review re-checked against the source | confirmed |

The verification harness is included as `sanitize-harness.cpp` if you want to
re-run it yourself: `clang++ -std=c++17 -Wall -Wextra -o harness sanitize-harness.cpp && ./harness`

### Not verified — needs a Windows box

I have no Windows machine and no access to the Go module proxy in this
environment, so the following are **reviewed but not compiled**:

1. **The C++ that touches Windows APIs** — `RemoveAttachmentsDirForStem`
   (`FindFirstFileW` enumeration), the `test_utils.cpp` change to
   `SHGetFolderPathW`, and `test_send_documents.cpp`. The logic is
   straightforward and the API usage is conventional, but treat the first
   successful build as the real check.
2. **The Go build.** `GOOS=windows go build` needs `proxy.golang.org`, which is
   blocked here. Changes are small and mechanical (deleting a dispatch block
   and two imports, changing two constants, editing a slice literal), and
   `gofmt -e` parses everything, but nothing has passed a compiler.
3. **The PowerShell test file.** No `pwsh` available; brace balance checked
   manually.

### First things to run on the build machine

```powershell
npm ci
npm run build:interceptor          # both x64 and x86 must succeed
npm run test                       # Go + Vitest
cd src\interceptor
.\build.ps1 -Arch x64 -Config Debug -Tests
cd ..\..
npm run test:interceptor           # the repaired harness, incl. MAPISendDocuments
```

Then the export-table check from section 8.1 of the runbook — the `__stdcall`
name-decoration footgun on the 32-bit DLL is the one that would silently break
your scanner software while everything else looks fine:

```powershell
llvm-nm --extern-only src\interceptor\build-x86\bin\go-mapi.dll | Select-String MAPI
```

---

## Deliberately not included

> **Update (ARRICKS-12, 2026-08-17):** the R-series below has been implemented
> as a second series — R3/R4/R9 in `internal/mapi` (queue-path containment,
> header-address CTL rejection, 18MB per-file + cumulative caps), R5 in the
> watcher (orphan-stem sweep with a 15-minute age floor), R7 in the
> interceptor (count caps + SEH guard where `__SEH__` is available), R10 in
> automode (permanent-failure retry cap; network-error classification fixed
> so refused/DNS failures count as transient), R11 as the
> `gomapi_debug_browser` build tag, and R12 by inverting credential
> precedence (ldflags win). The table is kept as the historical record of
> the original review.

These are in the review but out of scope for this pass. Each is small; say the
word and I will do them as a second series.

| ID | Item | Why it was left out |
|---|---|---|
| R3 | Constrain queue attachment paths to the queue dir (`gmail.go:144`) | ~5 lines, worth doing — needs a Go build to test |
| R4 | RFC-2047-encode the recipient address (`gmail.go:189`) | header-injection fix, same reason |
| R5 | Startup sweep for orphaned stem directories | the DLL-side leak is fixed; the Go-side sweep is not |
| R7 | Cap `nRecipCount`/`nFileCount`; `__try/__except` around conversion | touches the tested conversion path — deserves its own pass |
| R9 | Lower `MaxFileSize` to ~18MB, add a total-size check | trivial, but pairs naturally with R10 |
| R10 | Retry cap for permanent failures in Auto mode | needs care not to mask transient errors |
| R11 | Gate `GOMAPI_DEBUG_BROWSER_ARGS` behind a build tag | touches the vendored WebView2 fork |
| R12 | Make ldflags credentials win over env vars | trivial |
