# Agent Instructions: go-mapi

You are working on a specialized Windows bridge. The project is split into a **Low-Level Interceptor** and a **High-Level Client**.

### Technical Constraints
1. **The Bridge Protocol:**
   - The Interceptor and Client communicate via JSON files in `%TEMP%\go-mapi\incoming\`.
   - JSON Schema: `{ "subject": string, "body": string, "to": string[], "attachments": string[] }`.
   - File naming: `msg_[timestamp]_[random].json`.

2. **C++ Component (Interceptor):**
   - Goal: Smallest possible DLL. 
   - Use `nlohmann/json` (header-only) for serialization.
   - Target the **Simple MAPI** spec only. Do not attempt Extended MAPI/COM.
   - Primary exports: `MAPILogon`, `MAPILogoff`, `MAPISendMail`.

3. **Electron Component (Client):**
   - Use `chokidar` for high-performance file system watching.
   - Use `electron-store` for configuration.
   - Use `@googleapis/gmail` for Google integration.

### Persona Tasks
- **C++ Specialist:** Focus on `src/interceptor/`. Ensure the DLL is thread-safe and doesn't block the calling ERP application.
- **Frontend/Electron Guru:** Focus on `src/client/`. Build a clean, modern tray-to-Gmail flow. "Vibe coding" encouraged for the UI.
- **DevOps/CI Agent:** Focus on `.github/workflows/`. We need a Windows runner that compiles the C++ code using MSVC and packages the Electron app using `electron-builder`.

### Current Objective
Start by scaffolding the C++ `MAPISendMail` function that can successfully parse a `lpMessage` struct and write it to a local file.