# go-mapi Roadmap

This document outlines the strategic direction for go-mapi. We use phases rather than timelines - development pace depends on contributor availability.

## Phase 1: End-to-End Validation (Current)

**Goal:** Functional (not usable) product that validates our architecture

This phase proves the design works before investing in features. We don't process the email yet - we just verify data flows through all components.

### The Flow
1. User triggers **File → Send to → Mail Recipient** from Windows Explorer
2. Interceptor DLL captures the call and drops JSON in the dropbox folder
3. Native messaging host detects the file and reads it
4. Message arrives at the browser extension

### Edge Cases
- [ ] **Startup recovery** - Extension/browser/native-host was offline when interceptor ran; check dropbox on startup

### Testing Infrastructure
- [ ] **Playwright E2E enhancements** - Expand test coverage:
  - [ ] Multiple files sent rapidly (stress test)
  - [ ] Unicode filenames and content
  - [ ] Large attachments (mock paths)
  - [ ] Error recovery (corrupt JSON files)
- [ ] **Windows Sandbox for DLL testing** - Manual release gate:
  - [ ] Create `.wsb` configuration file
  - [ ] Setup script: register DLL, install extension, start native host
  - [ ] Test script: trigger "Send to → Mail recipient" via UI automation (AutoHotkey/PowerShell)
  - [ ] Document test matrix (Windows versions × scenarios)

### Foundation (Complete)
- [x] MAPI interceptor DLL (captures MAPISendMail)
- [x] JSON-based IPC between DLL and native host
- [x] Native messaging host (Go)
- [x] Browser extension scaffold (React)
- [x] Build system (npm scripts for all components)

### Success Criteria
- [ ] Can trigger send-to-mail from Explorer and see the message arrive in extension
- [ ] Extension logs show the file path that was "sent"
- [ ] Works even if extension was started after the send

## Phase 2: Core Functionality

**Goal:** Complete the basic MAPI-to-Gmail flow

### Features
- [ ] Gmail API integration (draft creation, sending)
- [ ] Basic extension UI for composing/reviewing email
- [ ] Error handling and user feedback
- [ ] Attachment support

## Phase 3: Production Ready

**Goal:** Make go-mapi ready for daily use

### Features
- [ ] **Settings UI** - Configure behavior in extension popup
- [ ] **MSI installer** - One-click installation for Windows
- [ ] **Auto-update** - Native host version checking

### Quality
- [ ] Comprehensive test coverage (unit + integration)
- [ ] Error logging and diagnostics
- [ ] Edge cases: large files, special characters, network failures

### Distribution
- [ ] Chrome Web Store listing
- [ ] Edge Add-ons listing
- [ ] GitHub Releases with MSI

## Phase 4: Enterprise Features

**Goal:** Support enterprise deployment scenarios

### Features
- [ ] **Group policies** - Configure via Windows GPO
- [ ] **Multiple accounts** - Select which Gmail account to use
- [ ] **Signature support** - Append configured signature
- [ ] **Template support** - Common email templates

### Deployment
- [ ] Silent MSI installation
- [ ] Enterprise extension deployment guide
- [ ] Intune/SCCM deployment documentation

## Phase 5: Extended Compatibility

**Goal:** Broader Windows and email provider support

### Windows
- [ ] Extended MAPI support (beyond Simple MAPI)
- [ ] Outlook integration considerations

### Email Providers
- [ ] Microsoft 365 / Outlook.com support
- [ ] Generic SMTP support
- [ ] Provider plugin architecture

## Deferred

Good ideas that aren't priorities yet:

- **Linux/macOS support** - Focus is Windows (where MAPI exists)
- **Mobile companion** - Notifications when emails arrive
- **Email scheduling** - Send later functionality
- **Read receipts** - Track email opens
- **Calendar integration** - Meeting request handling

## Dependencies

### Phase 2 requires:
- Phase 1 complete (end-to-end data flow validated)

### Phase 3 requires:
- Phase 2 complete (core Gmail flow working)
- Gmail API authentication stable

### Phase 4 requires:
- Phase 3 complete
- User feedback from production use
- Enterprise testing environment

### Phase 5 requires:
- Stable Phase 4
- Community interest in additional providers

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to contribute. The "Areas for Contribution" section lists prioritized tasks.

## Notes

- We don't commit to timelines - phases complete when they're ready
- Priorities may shift based on user feedback and contributor interest
- Enterprise features depend on actual enterprise users providing requirements
