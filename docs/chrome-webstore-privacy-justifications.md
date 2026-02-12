# Chrome Web Store - Justificaciones de Privacidad

## 1. SINGLE PURPOSE (Finalidad única)

**Descripción de la finalidad única:**

go-mapi enables Windows MAPI email functionality for Gmail users. It bridges the Windows "Send to → Mail recipient" feature with Gmail by intercepting MAPI calls, displaying them in the extension popup, and using the Gmail API to send emails. Single purpose: Make Gmail work as the default MAPI email handler on Windows.

**Versión en español:**

go-mapi habilita la funcionalidad de email MAPI de Windows para usuarios de Gmail. Conecta la función de Windows "Enviar a → Destinatario de correo" con Gmail interceptando llamadas MAPI, mostrándolas en el popup de la extensión, y usando la API de Gmail para enviar correos. Finalidad única: Hacer que Gmail funcione como manejador de email MAPI predeterminado en Windows.

---

## 2. PERMISSIONS JUSTIFICATIONS (Justificaciones de permisos)

### ALARMS

**Justification:**

The alarms permission is used to periodically check for new MAPI email requests from the native host. The extension sets an alarm to poll for updates every few seconds to ensure emails sent via Windows applications appear promptly in the extension popup.

**Spanish:**

El permiso alarms se usa para verificar periódicamente nuevas solicitudes de email MAPI desde el host nativo. La extensión configura una alarma para consultar actualizaciones cada pocos segundos y asegurar que los emails enviados desde aplicaciones Windows aparezcan rápidamente en el popup.

---

### IDENTITY

**Justification:**

The identity permission is required to authenticate users with their Google/Gmail accounts using Chrome's OAuth flow. This permission enables the extension to obtain OAuth tokens to access the Gmail API for sending emails on behalf of the user. No credentials are stored by the extension.

**Spanish:**

El permiso identity es necesario para autenticar usuarios con sus cuentas de Google/Gmail usando el flujo OAuth de Chrome. Este permiso permite a la extensión obtener tokens OAuth para acceder a la API de Gmail y enviar correos en nombre del usuario. La extensión no almacena credenciales.

---

### NATIVEMESSAGING

**Justification:**

The nativeMessaging permission allows the extension to communicate with a locally installed native application (go-mapi-host.exe) that watches for MAPI email requests from Windows applications. This is the core functionality - bridging Windows MAPI calls to the browser extension.

**Spanish:**

El permiso nativeMessaging permite a la extensión comunicarse con una aplicación nativa instalada localmente (go-mapi-host.exe) que monitorea solicitudes de email MAPI desde aplicaciones Windows. Esta es la funcionalidad principal: conectar llamadas MAPI de Windows con la extensión del navegador.

---

### NOTIFICATIONS

**Justification:**

The notifications permission is used to show desktop notifications when new MAPI email requests arrive, informing the user that an email is ready to be reviewed and sent. This improves user experience by alerting them when action is needed.

**Spanish:**

El permiso notifications se usa para mostrar notificaciones de escritorio cuando llegan nuevas solicitudes de email MAPI, informando al usuario que hay un correo listo para revisar y enviar. Esto mejora la experiencia alertando cuando se requiere acción.

---

### STORAGE

**Justification:**

The storage permission is used to save user preferences such as default action (save as draft vs. send immediately), notification settings, and the last known state of pending emails. All data is stored locally in the browser and never transmitted to external servers.

**Spanish:**

El permiso storage se usa para guardar preferencias del usuario como la acción predeterminada (guardar como borrador vs. enviar inmediatamente), configuración de notificaciones, y el último estado conocido de emails pendientes. Todos los datos se almacenan localmente en el navegador y nunca se transmiten a servidores externos.

---

## 3. HOST PERMISSIONS (Permisos de host)

### https://www.googleapis.com/*

**Justification:**

This host permission is required to access the Gmail API endpoints for sending emails and creating drafts. The extension only communicates with Google's official Gmail API servers to perform email operations explicitly requested by the user (clicking "Send" or "Save as Draft").

**Spanish:**

Este permiso de host es necesario para acceder a los endpoints de la API de Gmail para enviar correos y crear borradores. La extensión solo se comunica con los servidores oficiales de la API de Gmail para realizar operaciones de email explícitamente solicitadas por el usuario (clic en "Enviar" o "Guardar como borrador").

---

## 4. REMOTE CODE (Código remoto)

### Does your extension use remote code?

**Answer: NO**

**Justification:**

The extension does not execute any remote code. All code is bundled within the extension package and runs locally. The extension only communicates with:
1. A local native application via Chrome's Native Messaging API
2. Google's Gmail API (via standard HTTPS requests to googleapis.com)

No external scripts, libraries, or code are loaded or executed at runtime.

**Spanish:**

La extensión no ejecuta código remoto. Todo el código está incluido en el paquete de la extensión y se ejecuta localmente. La extensión solo se comunica con:
1. Una aplicación nativa local vía la API de Mensajería Nativa de Chrome
2. La API de Gmail de Google (vía solicitudes HTTPS estándar a googleapis.com)

No se cargan ni ejecutan scripts, librerías o código externo en tiempo de ejecución.

---

## 5. DATA USAGE CERTIFICATION (Certificación de uso de datos)

### Do you collect user data?

**Answer: NO**

**Justification:**

go-mapi does not collect, transmit, or store any user data on external servers. All data processing happens locally:

- MAPI email requests are temporarily stored in Windows temp folder
- Files are deleted after the email is sent or dismissed
- User preferences are stored in Chrome's local storage
- OAuth tokens are managed by Chrome's identity system
- No analytics, telemetry, or tracking of any kind

The only external communication is with Gmail API when the user explicitly clicks "Send" or "Save as Draft".

**Spanish:**

go-mapi no recopila, transmite ni almacena datos de usuario en servidores externos. Todo el procesamiento de datos ocurre localmente:

- Las solicitudes de email MAPI se almacenan temporalmente en la carpeta temp de Windows
- Los archivos se eliminan después de que el email es enviado o descartado
- Las preferencias de usuario se guardan en el almacenamiento local de Chrome
- Los tokens OAuth son gestionados por el sistema de identidad de Chrome
- No hay análisis, telemetría ni seguimiento de ningún tipo

La única comunicación externa es con la API de Gmail cuando el usuario hace clic explícitamente en "Enviar" o "Guardar como borrador".

---

## 6. GMAIL API SCOPES (Alcances de la API de Gmail)

The extension only requests these minimal Gmail API scopes:

### gmail.compose
To create new draft emails in the user's Gmail account.

### gmail.send
To send emails approved by the user.

**Important:** The extension CANNOT read existing emails, access inbox, or view any messages. It only has permission to create and send new emails.

---

## 7. PRIVACY POLICY URL

https://github.com/marcfargas/go-mapi/blob/main/PRIVACY.md

(You should create this file - I can help with that next)

---

## 8. SUPPORT/CONTACT EMAIL

You need to provide a support email. Suggestions:
- support@go-mapi.org
- gomapi.support@gmail.com
- [your email]

---

## 9. SCREENSHOTS NEEDED

The Web Store requires at least 1 screenshot. I recommend:

1. **Main popup with email** (1280x800 or 640x400)
   - Show the extension popup with a sample email
   - File attachment visible
   - "Save as Draft" and "Send Now" buttons

2. **Windows Explorer context menu** (1280x800)
   - Right-click on file
   - "Send to → Mail recipient" highlighted

3. **Success notification** (640x400)
   - Desktop notification showing "Email sent successfully"

4. **Settings panel** (640x400)
   - Extension settings/preferences screen

Need me to help create these screenshots?

---

## SUMMARY CHECKLIST

Copy these justifications to the Chrome Web Store "Privacy practices" tab:

- [ ] Single purpose description
- [ ] alarms permission justification
- [ ] identity permission justification  
- [ ] nativeMessaging permission justification
- [ ] notifications permission justification
- [ ] storage permission justification
- [ ] Host permission justification (googleapis.com)
- [ ] Remote code justification (answer: NO)
- [ ] Data collection certification (answer: NO)
- [ ] Support email address
- [ ] Privacy policy URL
- [ ] At least 1 screenshot uploaded
