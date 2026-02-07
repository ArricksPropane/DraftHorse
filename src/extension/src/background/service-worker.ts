import {
  MSG_TYPE,
  type NativeIncomingMessage,
  type NativeOutgoingMessage,
  type EmailWithId,
  type ExtensionMessage,
} from '../types/messages';
import { createDraft, getAuthToken } from '../lib/gmail';

const NATIVE_HOST = 'com.gomapi.host';
const RECONNECT_ALARM = 'reconnect';

// State
let nativePort: chrome.runtime.Port | null = null;
let emails: Map<string, EmailWithId> = new Map();
let isConnected = false;
let hostVersion = '';

// Track pending uploads: draftId -> { resolve, reject, emailId }
const pendingUploads: Map<string, {
  resolve: (draftId: string) => void;
  reject: (error: string) => void;
  emailId: string;
}> = new Map();

// --- Persistence helpers ---

async function persistEmails() {
  const obj: Record<string, EmailWithId> = {};
  for (const [k, v] of emails) {
    obj[k] = v;
  }
  await chrome.storage.session.set({ emails: obj });
}

async function loadEmails() {
  const result = await chrome.storage.session.get('emails');
  if (result.emails) {
    emails = new Map(Object.entries(result.emails as Record<string, EmailWithId>));
  }
}

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

      // Reject any pending uploads
      for (const [draftId, pending] of pendingUploads) {
        pending.reject('Native host disconnected');
        pendingUploads.delete(draftId);
      }

      // Use chrome.alarms for reconnection — survives MV3 worker termination
      chrome.alarms.create(RECONNECT_ALARM, { delayInMinutes: 0.1 }); // ~6 seconds
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

// Alarm listener for reconnection
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === RECONNECT_ALARM && !nativePort) {
    connectToNativeHost();
  }
});

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

    case MSG_TYPE.EMAIL: {
      const emailWithId: EmailWithId = {
        ...message.data,
        id: message.id,
      };
      emails.set(message.id, emailWithId);
      persistEmails();
      updateBadge();
      broadcastToPopup({ type: 'QUEUE_UPDATE', emails: Array.from(emails.values()) });
      break;
    }

    case MSG_TYPE.REMOVED:
      emails.delete(message.id);
      persistEmails();
      updateBadge();
      broadcastToPopup({ type: 'QUEUE_UPDATE', emails: Array.from(emails.values()) });
      break;

    case MSG_TYPE.ERROR:
      console.error('[go-mapi] Host error:', message.error);
      broadcastToPopup({ type: 'ERROR', error: message.error });
      break;

    case MSG_TYPE.UPLOAD_COMPLETE: {
      const pending = pendingUploads.get(message.draftId);
      if (pending) {
        pendingUploads.delete(message.draftId);
        pending.resolve(message.draftId);
      }
      break;
    }

    case MSG_TYPE.UPLOAD_ERROR: {
      const pending = pendingUploads.get(message.draftId);
      if (pending) {
        pendingUploads.delete(message.draftId);
        pending.reject(message.error);
      }
      break;
    }

    case MSG_TYPE.UPLOAD_PROGRESS:
      console.log(`[go-mapi] Upload progress: ${message.current}/${message.total} — ${message.filename}`);
      broadcastToPopup({
        type: 'UPLOAD_PROGRESS',
        draftId: message.draftId,
        current: message.current,
        total: message.total,
        filename: message.filename,
      });
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

// Upload attachments to a Gmail draft via the Go host
function uploadAttachments(
  draftId: string,
  messageId: string,
  token: string,
  attachments: { path: string; filename: string }[],
  emailId: string,
): Promise<string> {
  return new Promise((resolve, reject) => {
    pendingUploads.set(draftId, { resolve, reject, emailId });

    sendToNativeHost({
      type: MSG_TYPE.UPLOAD_ATTACHMENTS,
      draftId,
      messageId,
      token,
      attachments: attachments.map(a => ({ path: a.path, filename: a.filename })),
    });
  });
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

          // Create text-only draft first
          const draftId = await createDraft(email);

          // Mark as processed in the file watcher
          sendToNativeHost({ type: MSG_TYPE.PROCESS, id: request.id });
          emails.delete(request.id);
          await persistEmails();
          updateBadge();
          broadcastToPopup({ type: 'QUEUE_UPDATE', emails: Array.from(emails.values()) });

          // If email has attachments, upload them via the Go host
          if (email.attachments && email.attachments.length > 0) {
            try {
              const token = await getAuthToken();
              // draftId from createDraft is the draft resource ID
              // We need the message ID too — re-fetch isn't needed,
              // the Go host will GET the draft to get the raw message
              await uploadAttachments(
                draftId,
                '', // messageId not needed — host fetches draft by draftId
                token,
                email.attachments,
                request.id,
              );
            } catch (uploadError) {
              // Draft was created but attachments failed
              // Still open Gmail so user can manually attach
              console.error('[go-mapi] Attachment upload failed:', uploadError);
              broadcastToPopup({
                type: 'ERROR',
                error: `Draft created but attachment upload failed: ${uploadError}. You can manually attach files in Gmail.`,
              });
            }
          }

          // Open Gmail to the draft
          chrome.tabs.create({
            url: `https://mail.google.com/mail/u/0/#drafts?compose=${draftId}`,
          });
          sendResponse({ success: true, draftId });
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

// Initialize — restore persisted state, then connect
console.log('[go-mapi] Service worker starting');
loadEmails().then(() => {
  updateBadge();
  connectToNativeHost();
});
