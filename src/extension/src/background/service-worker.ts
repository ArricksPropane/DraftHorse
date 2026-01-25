import {
  MSG_TYPE,
  type NativeIncomingMessage,
  type NativeOutgoingMessage,
  type EmailWithId,
  type MailMessage,
  type ExtensionMessage,
} from '../types/messages';

const NATIVE_HOST = 'com.gomapi.host';

// State
let nativePort: chrome.runtime.Port | null = null;
let emails: Map<string, EmailWithId> = new Map();
let isConnected = false;
let hostVersion = '';

// Badge update
function updateBadge() {
  const count = emails.size;
  chrome.action.setBadgeText({ text: count > 0 ? String(count) : '' });
  chrome.action.setBadgeBackgroundColor({ color: '#0d6efd' });
}

// Broadcast to popup
function broadcastToPopup(message: ExtensionMessage) {
  chrome.runtime.sendMessage(message).catch(() => {
    // Popup not open, ignore
  });
}

// Connect to native host
function connectToNativeHost() {
  if (nativePort) {
    return;
  }

  console.log('[go-mapi] Connecting to native host...');

  try {
    nativePort = chrome.runtime.connectNative(NATIVE_HOST);

    nativePort.onMessage.addListener((message: NativeIncomingMessage) => {
      console.log('[go-mapi] Received:', message);
      handleNativeMessage(message);
    });

    nativePort.onDisconnect.addListener(() => {
      const error = chrome.runtime.lastError?.message || 'Unknown error';
      console.log('[go-mapi] Disconnected:', error);
      nativePort = null;
      isConnected = false;
      broadcastToPopup({ type: 'CONNECTION_STATUS', connected: false, error });

      // Retry connection after delay
      setTimeout(connectToNativeHost, 5000);
    });
  } catch (error) {
    console.error('[go-mapi] Failed to connect:', error);
    isConnected = false;
    broadcastToPopup({
      type: 'CONNECTION_STATUS',
      connected: false,
      error: String(error)
    });
  }
}

// Handle messages from native host
function handleNativeMessage(message: NativeIncomingMessage) {
  switch (message.type) {
    case MSG_TYPE.READY:
      isConnected = true;
      hostVersion = message.version;
      console.log('[go-mapi] Host ready, version:', hostVersion);
      broadcastToPopup({ type: 'CONNECTION_STATUS', connected: true });
      // Request current emails
      sendToNativeHost({ type: MSG_TYPE.LIST });
      break;

    case MSG_TYPE.EMAIL:
      const emailWithId: EmailWithId = {
        ...message.data,
        id: message.id,
      };
      emails.set(message.id, emailWithId);
      updateBadge();
      broadcastToPopup({ type: 'QUEUE_UPDATE', emails: Array.from(emails.values()) });
      break;

    case MSG_TYPE.REMOVED:
      emails.delete(message.id);
      updateBadge();
      broadcastToPopup({ type: 'QUEUE_UPDATE', emails: Array.from(emails.values()) });
      break;

    case MSG_TYPE.ERROR:
      console.error('[go-mapi] Host error:', message.error);
      broadcastToPopup({ type: 'ERROR', error: message.error });
      break;
  }
}

// Send message to native host
function sendToNativeHost(message: NativeOutgoingMessage) {
  if (!nativePort) {
    console.warn('[go-mapi] Not connected to native host');
    return;
  }
  console.log('[go-mapi] Sending:', message);
  nativePort.postMessage(message);
}

// Gmail API helpers
async function getAuthToken(): Promise<string> {
  return new Promise((resolve, reject) => {
    chrome.identity.getAuthToken({ interactive: true }, (token) => {
      if (chrome.runtime.lastError) {
        reject(new Error(chrome.runtime.lastError.message));
      } else if (token) {
        resolve(token);
      } else {
        reject(new Error('No token received'));
      }
    });
  });
}

function buildRfc2822Message(email: MailMessage): string {
  const lines: string[] = [];

  // From will be filled by Gmail
  // To
  if (email.recipients.to.length > 0) {
    const to = email.recipients.to
      .map((r) => (r.name ? `"${r.name}" <${r.address}>` : r.address))
      .join(', ');
    lines.push(`To: ${to}`);
  }

  // CC
  if (email.recipients.cc.length > 0) {
    const cc = email.recipients.cc
      .map((r) => (r.name ? `"${r.name}" <${r.address}>` : r.address))
      .join(', ');
    lines.push(`Cc: ${cc}`);
  }

  // BCC
  if (email.recipients.bcc.length > 0) {
    const bcc = email.recipients.bcc
      .map((r) => (r.name ? `"${r.name}" <${r.address}>` : r.address))
      .join(', ');
    lines.push(`Bcc: ${bcc}`);
  }

  // Subject
  lines.push(`Subject: ${email.subject || '(No Subject)'}`);

  // Content-Type
  if (email.bodyFormat === 'html') {
    lines.push('Content-Type: text/html; charset=UTF-8');
  } else {
    lines.push('Content-Type: text/plain; charset=UTF-8');
  }

  // Empty line before body
  lines.push('');

  // Body
  lines.push(email.body || '');

  return lines.join('\r\n');
}

function base64UrlEncode(str: string): string {
  const utf8Bytes = new TextEncoder().encode(str);
  const base64 = btoa(String.fromCharCode(...utf8Bytes));
  return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

async function createDraft(email: MailMessage): Promise<string> {
  const token = await getAuthToken();
  const rawMessage = buildRfc2822Message(email);
  const encodedMessage = base64UrlEncode(rawMessage);

  const response = await fetch('https://www.googleapis.com/gmail/v1/users/me/drafts', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      message: {
        raw: encodedMessage,
      },
    }),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(`Failed to create draft: ${error}`);
  }

  const data = await response.json();
  return data.id;
}

async function sendEmail(email: MailMessage): Promise<string> {
  const token = await getAuthToken();
  const rawMessage = buildRfc2822Message(email);
  const encodedMessage = base64UrlEncode(rawMessage);

  const response = await fetch('https://www.googleapis.com/gmail/v1/users/me/messages/send', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      raw: encodedMessage,
    }),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(`Failed to send email: ${error}`);
  }

  const data = await response.json();
  return data.id;
}

// Message handlers from popup
chrome.runtime.onMessage.addListener((request, _sender, sendResponse) => {
  console.log('[go-mapi] Popup message:', request);

  (async () => {
    try {
      switch (request.action) {
        case 'getEmails':
          sendResponse({
            success: true,
            emails: Array.from(emails.values()),
            connected: isConnected,
          });
          break;

        case 'createDraft': {
          const email = emails.get(request.id);
          if (!email) {
            sendResponse({ success: false, error: 'Email not found' });
            return;
          }
          const draftId = await createDraft(email);
          // Mark as processed
          sendToNativeHost({ type: MSG_TYPE.PROCESS, id: request.id });
          emails.delete(request.id);
          updateBadge();
          broadcastToPopup({ type: 'QUEUE_UPDATE', emails: Array.from(emails.values()) });
          // Open Gmail to the draft
          chrome.tabs.create({
            url: `https://mail.google.com/mail/u/0/#drafts?compose=${draftId}`,
          });
          sendResponse({ success: true, draftId });
          break;
        }

        case 'sendEmail': {
          const email = emails.get(request.id);
          if (!email) {
            sendResponse({ success: false, error: 'Email not found' });
            return;
          }
          const messageId = await sendEmail(email);
          // Mark as processed
          sendToNativeHost({ type: MSG_TYPE.PROCESS, id: request.id });
          emails.delete(request.id);
          updateBadge();
          broadcastToPopup({ type: 'QUEUE_UPDATE', emails: Array.from(emails.values()) });
          sendResponse({ success: true, messageId });
          break;
        }

        case 'deleteEmail':
          sendToNativeHost({ type: MSG_TYPE.DELETE, id: request.id });
          sendResponse({ success: true });
          break;

        case 'reconnect':
          if (!nativePort) {
            connectToNativeHost();
          }
          sendResponse({ success: true });
          break;

        default:
          sendResponse({ success: false, error: 'Unknown action' });
      }
    } catch (error) {
      console.error('[go-mapi] Error:', error);
      sendResponse({ success: false, error: String(error) });
    }
  })();

  return true; // Keep channel open for async response
});

// Initialize
console.log('[go-mapi] Service worker starting');
connectToNativeHost();
updateBadge();
