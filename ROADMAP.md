# go-mapi Roadmap

This document outlines the strategic direction for go-mapi. We use phases rather than timelines - development pace depends on contributor availability.

## Current Phase: Core Functionality

**Goal:** Complete the basic MAPI-to-Gmail flow

### In Progress
- [ ] Gmail API integration (draft creation, sending)
- [ ] Basic extension UI refinements
- [ ] Error handling and user feedback

### Recently Completed
- [x] MAPI interceptor DLL (captures MAPISendMail)
- [x] JSON-based IPC between DLL and native host
- [x] Native messaging host (Go)
- [x] Browser extension scaffold (React)
- [x] Build system (npm scripts for all components)

## Phase 2: Production Ready

**Goal:** Make go-mapi ready for daily use

### Features
- [ ] **Attachment support** - Upload files to Gmail via extension
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

## Phase 3: Enterprise Features

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

## Phase 4: Extended Compatibility

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
- Core flow working end-to-end
- Gmail API authentication stable

### Phase 3 requires:
- Phase 2 complete
- User feedback from production use
- Enterprise testing environment

### Phase 4 requires:
- Stable Phase 3
- Community interest in additional providers

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to contribute. The "Areas for Contribution" section lists prioritized tasks.

## Notes

- We don't commit to timelines - phases complete when they're ready
- Priorities may shift based on user feedback and contributor interest
- Enterprise features depend on actual enterprise users providing requirements
