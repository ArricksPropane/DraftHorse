# Chrome Web Store - Testing Instructions for Reviewers

## TESTING INSTRUCTIONS (English)

**Note for Reviewers:** This extension requires additional Windows components to function fully. However, the extension UI and core functionality can be tested without them.

### Option 1: Basic UI Testing (No Installation Required)

The extension popup and UI can be reviewed without installing the native components:

1. Install the extension in Chrome
2. Click the extension icon in the toolbar
3. The popup will open showing the "Waiting for emails..." state
4. Review the UI, permissions, and code

**What you'll see:**
- Empty state UI with instructions
- Settings panel (click gear icon)
- Clean, functional React-based interface

**Limitations:** 
- No emails will appear (requires Windows native host)
- Gmail sending cannot be tested (requires OAuth + native host)

### Option 2: Full Testing (Windows 10/11 Required)

To test the complete MAPI-to-Gmail bridge functionality:

#### Prerequisites:
- Windows 10 or Windows 11
- Gmail or Google Workspace test account
- Administrator access

#### Installation Steps:

1. **Install the extension** in Chrome and note the Extension ID
   - Go to chrome://extensions/
   - Enable "Developer mode"
   - Copy the extension ID (long alphanumeric string)

2. **Run the installer** (PowerShell as Administrator):
   ```powershell
   irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/install.ps1 | iex
   ```
   - When prompted, paste the extension ID
   - The installer will download and configure all components

3. **Authenticate with Gmail**:
   - Click the extension icon
   - Click "Sign in with Google"
   - Grant permissions when prompted

4. **Test the MAPI bridge**:
   - Right-click any file in Windows Explorer
   - Select "Send to → Mail recipient"
   - The email should appear in the extension popup
   - Click "Save as Draft" or "Send Now"

#### Alternative: Manual Component Download

If the automated installer doesn't work:

1. Download the latest release from: https://github.com/marcfargas/go-mapi/releases
2. Extract to `C:\Program Files\go-mapi\`
3. Run the installer script with `-Local` flag

### Option 3: Code Review Only

You can review the extension's source code without installation:

- GitHub repository: https://github.com/marcfargas/go-mapi
- Extension code: `/src/extension/` directory
- manifest.json: `/src/extension/public/manifest.json`

All code is open source and auditable.

---

## INSTRUCCIONES DE PRUEBA (Español)

**Nota para Revisores:** Esta extensión requiere componentes adicionales de Windows para funcionar completamente. Sin embargo, la UI y funcionalidad principal pueden probarse sin ellos.

### Opción 1: Prueba Básica de UI (Sin Instalación Requerida)

El popup y la interfaz de la extensión pueden revisarse sin instalar los componentes nativos:

1. Instalar la extensión en Chrome
2. Hacer clic en el icono de la extensión en la barra de herramientas
3. El popup se abrirá mostrando el estado "Esperando correos..."
4. Revisar la UI, permisos y código

**Lo que verá:**
- UI de estado vacío con instrucciones
- Panel de configuración (clic en icono de engranaje)
- Interfaz limpia y funcional basada en React

**Limitaciones:**
- No aparecerán correos (requiere host nativo de Windows)
- No se puede probar el envío a Gmail (requiere OAuth + host nativo)

### Opción 2: Prueba Completa (Windows 10/11 Requerido)

Para probar la funcionalidad completa del puente MAPI-a-Gmail:

#### Requisitos previos:
- Windows 10 o Windows 11
- Cuenta de prueba de Gmail o Google Workspace
- Acceso de administrador

#### Pasos de instalación:

1. **Instalar la extensión** en Chrome y anotar el Extension ID
   - Ir a chrome://extensions/
   - Activar "Modo de desarrollador"
   - Copiar el extension ID (cadena alfanumérica larga)

2. **Ejecutar el instalador** (PowerShell como Administrador):
   ```powershell
   irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/install.ps1 | iex
   ```
   - Cuando se solicite, pegar el extension ID
   - El instalador descargará y configurará todos los componentes

3. **Autenticar con Gmail**:
   - Hacer clic en el icono de la extensión
   - Hacer clic en "Sign in with Google"
   - Otorgar permisos cuando se solicite

4. **Probar el puente MAPI**:
   - Clic derecho en cualquier archivo en el Explorador de Windows
   - Seleccionar "Enviar a → Destinatario de correo"
   - El correo debería aparecer en el popup de la extensión
   - Hacer clic en "Save as Draft" o "Send Now"

#### Alternativa: Descarga Manual de Componentes

Si el instalador automático no funciona:

1. Descargar la última versión de: https://github.com/marcfargas/go-mapi/releases
2. Extraer a `C:\Program Files\go-mapi\`
3. Ejecutar el script de instalación con el flag `-Local`

### Opción 3: Solo Revisión de Código

Puede revisar el código fuente de la extensión sin instalación:

- Repositorio GitHub: https://github.com/marcfargas/go-mapi
- Código de extensión: directorio `/src/extension/`
- manifest.json: `/src/extension/public/manifest.json`

Todo el código es de código abierto y auditable.

---

## QUICK SUMMARY FOR FORM FIELD

**Copy this into the Web Store "Testing instructions" field:**

This extension requires a Windows native host component for full functionality. Reviewers have three options:

1. BASIC TESTING (No setup): Install extension, click icon, review UI (will show "Waiting for emails..." state)

2. FULL TESTING (Windows required): Run installer: irm https://raw.githubusercontent.com/marcfargas/go-mapi/main/scripts/install.ps1 | iex (PowerShell as Admin), paste extension ID when prompted, authenticate with test Gmail account, right-click any file → Send to → Mail recipient to test.

3. CODE REVIEW: Source code available at https://github.com/marcfargas/go-mapi/tree/main/src/extension

For basic UI review, no additional setup beyond extension installation is needed. The popup will display even without the native host.

---

## TEST ACCOUNT (If Required)

If the Web Store requires a test Gmail account:

**Email:** [Your test email]
**Password:** [Your test password]

**Note:** A real Gmail account is needed to test OAuth and Gmail API integration. The account should have:
- Gmail enabled
- OAuth consent screen configured (or use personal account for testing)

---

## ADDITIONAL NOTES FOR REVIEWERS

### Privacy & Security:
- Extension does NOT collect user data
- No external servers except Gmail API
- OAuth tokens managed by Chrome's identity system
- All code is open source and auditable on GitHub

### Permissions Explained:
- **nativeMessaging**: Communicate with local Windows service
- **identity**: Gmail OAuth authentication
- **storage**: Save user preferences locally
- **notifications**: Alert when emails arrive
- **alarms**: Periodic check for new MAPI requests
- **googleapis.com**: Access Gmail API to send emails

### Why Native Host is Required:
Windows MAPI (Messaging Application Programming Interface) is a system-level API. A browser extension cannot directly intercept MAPI calls. The native host:
1. Registers a DLL that intercepts MAPI calls
2. Watches for email requests from Windows apps
3. Sends them to the extension via Chrome Native Messaging

This architecture is necessary and follows Chrome's best practices for native integration.

---

## SUPPORT CONTACT

For reviewer questions:
- GitHub Issues: https://github.com/marcfargas/go-mapi/issues
- Email: [Your support email]
- Documentation: https://github.com/marcfargas/go-mapi
