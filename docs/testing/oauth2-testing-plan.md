# OAuth2 Testing Plan

**When:** After Google Cloud OAuth2 setup is complete (TODO-25d0943c)

## Prerequisites

- [ ] Google Cloud project with Gmail API enabled
- [ ] OAuth consent screen configured (test mode is fine)
- [ ] Chrome App client ID created
- [ ] Client ID plugged into `src/extension/public/manifest.json`
- [ ] Your Google account added as test user in consent screen
- [ ] Extension loaded in Chrome/Edge (`chrome://extensions` → Load unpacked → `src/extension/dist/`)
- [ ] Go native host built and registered

---

## Test 1: Basic Token Acquisition

**Goal:** Verify `chrome.identity.getAuthToken()` works with the real client ID.

1. Load extension in Chrome/Edge
2. Open popup → click any action that triggers `getAuthToken()`
3. Should see Google consent screen on first use
4. Accept → token acquired → no errors in DevTools console

**Pass criteria:** Token acquired, no errors.

---

## Test 2: Draft Creation (No Attachments)

**Goal:** End-to-end draft creation without attachments.

1. Drop test JSON into `%TEMP%\go-mapi\` (use `scripts/test-drop-email.ps1`)
2. Email appears in popup
3. Click "Save as Draft"

**Pass criteria:**
- Draft appears in Gmail Drafts
- Correct subject, body, recipients
- Gmail tab opens to the draft

**Test JSON:**
```json
{
  "version": 1,
  "timestamp": "2026-02-07T21:00:00.000Z",
  "subject": "OAuth test — no attachments",
  "body": "Testing draft creation via go-mapi.",
  "bodyFormat": "plain",
  "recipients": {"to": [{"name": "Test", "address": "your@email.com"}], "cc": [], "bcc": []},
  "attachments": [],
  "originApp": "manual-test"
}
```

---

## Test 3: Draft with Attachments (Core Flow)

**Goal:** Full attachment upload pipeline: extension → Go host → Gmail API.

1. Drop test JSON with real attachment paths (use `scripts/test-drop-email.ps1 -WithAttachment`)
2. Click "Save as Draft"
3. Watch the flow:
   - Draft created (text-only)
   - Extension sends `upload-attachments` to Go host
   - Go host GETs draft, rebuilds MIME with attachments, PUTs back
   - `upload-complete` received
   - Gmail tab opens

**Pass criteria:**
- Draft in Gmail has attachment(s) attached
- `%TEMP%\go-mapi\native-host.log` shows successful GET/PUT (no 401)

**Test JSON:**
```json
{
  "version": 1,
  "timestamp": "2026-02-07T21:00:00.000Z",
  "subject": "OAuth test — with attachment",
  "body": "Testing attachment upload via go-mapi.",
  "bodyFormat": "plain",
  "recipients": {"to": [{"name": "Test", "address": "your@email.com"}], "cc": [], "bcc": []},
  "attachments": [
    {"filename": "test.pdf", "path": "C:\\path\\to\\real\\file.pdf", "size": 0}
  ],
  "originApp": "manual-test"
}
```

> **Note:** Update the attachment `path` to a real file on your machine.

---

## Test 4: Token Passed to Go Host

**Goal:** Verify the OAuth token works when the Go host calls Gmail API directly.

Implicitly tested by Test 3. Specifically verify:
- Go host log shows successful GET and PUT to Gmail API
- No 401 errors in the log
- Check: `type native-host.log` in `%TEMP%\go-mapi\`

---

## Test 5: Error Cases

### 5a: Attachment file doesn't exist
- Use a JSON with a non-existent path: `"path": "C:\\nonexistent\\file.pdf"`
- **Expected:** Error message in popup, draft still opens in Gmail (without attachment)

### 5b: Revoked access
- Revoke app access in [Google Account settings](https://myaccount.google.com/permissions)
- Try "Save as Draft"
- **Expected:** Auth error, user prompted to re-authenticate

### 5c: Large file (>25MB)
- Use an attachment path pointing to a file >25MB
- **Expected:** Clear error message about size limit

### 5d: No recipients
- Drop JSON with empty recipients: `"recipients": {"to": [], "cc": [], "bcc": []}`
- Click "Save as Draft"
- **Expected:** Draft created successfully (Gmail allows drafts without recipients)

---

## Test 6: Token Refresh

**Goal:** Verify tokens refresh automatically after expiry.

1. Complete Test 2 or 3 successfully
2. Wait ~1 hour (or revoke and re-grant)
3. Try "Save as Draft" again
4. `chrome.identity.getAuthToken()` should silently refresh

**Pass criteria:** Draft creation works without re-prompting consent.

**Known limitation (post-MVP):** If the Go host gets a 401 mid-upload, the error surfaces to the extension. Automatic retry with fresh token is Phase 3 (TODO: OAuth token refresh/retry on 401).

---

## Test Helper Script

Use `scripts/test-drop-email.ps1` to drop test emails:

```powershell
# Simple email (no attachments)
.\scripts\test-drop-email.ps1

# Email with attachment
.\scripts\test-drop-email.ps1 -WithAttachment -AttachmentPath "C:\path\to\file.pdf"

# Email with multiple recipients
.\scripts\test-drop-email.ps1 -CC "cc@example.com" -BCC "bcc@example.com"
```

---

## Debugging Checklist

If something fails:

1. **Extension DevTools:** Right-click extension icon → Inspect popup. Check Console and Network tabs.
2. **Service worker logs:** `chrome://extensions` → go-mapi → "Inspect views: service worker"
3. **Native host log:** `%TEMP%\go-mapi\native-host.log`
4. **JSON files:** Check `%TEMP%\go-mapi\` for pending/error files
5. **Gmail API errors:** Look for HTTP status codes in native host log (401 = token, 403 = permissions, 400 = bad request)
