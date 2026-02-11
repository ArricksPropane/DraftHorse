import {
  MSG_TYPE,
  type NativeIncomingMessage,
  type NativeOutgoingMessage,
  type EmailWithId,
  type ExtensionMessage,
  type RecentDraft,
} from '../types/messages';
import { createDraft, getAuthToken } from '../lib/gmail';

const NATIVE_HOST = 'com.gomapi.host';
const RECONNECT_ALARM = 'reconnect';

// State
let nativePort: chrome.runtime.Port | null = null;
let emails: Map<string, EmailWithId> = new Map();
let isConnected = false;
let hostVersion = '';

// Recent drafts shown in popup
let recentDrafts: RecentDraft[] = [];

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

async function persistDrafts() {
  // Keep last 20 drafts max
  if (recentDrafts.length > 20) {
    recentDrafts = recentDrafts.slice(0, 20);
  }
  await chrome.storage.session.set({ recentDrafts });
}

async function loadEmails() {
  const result = await chrome.storage.session.get(['emails', 'recentDrafts']);
  if (result.emails) {
    emails = new Map(Object.entries(result.emails as Record<string, EmailWithId>));
  }
  if (result.recentDrafts) {
    recentDrafts = result.recentDrafts as RecentDraft[];
  }
}

// Badge update — show pending emails count (red) or drafts count (blue)
function updateBadge() {
  const pending = emails.size;
  if (pending > 0) {
    chrome.action.setBadgeText({ text: String(pending) });
    chrome.action.setBadgeBackgroundColor({ color: '#dc3545' }); // red = needs attention
  } else if (recentDrafts.length > 0) {
    chrome.action.setBadgeText({ text: String(recentDrafts.length) });
    chrome.action.setBadgeBackgroundColor({ color: '#0d6efd' }); // blue = drafts ready
  } else {
    chrome.action.setBadgeText({ text: '' });
  }
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

      // Auto-create Gmail draft and show notification
      autoCreateDraft(emailWithId);
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

// --- Auto-draft and notification ---

async function autoCreateDraft(email: EmailWithId) {
  try {
    // Try non-interactive auth first — don't force browser sign-in
    try {
      await new Promise<string>((resolve, reject) => {
        chrome.identity.getAuthToken({ interactive: false }, (t) => {
          if (chrome.runtime.lastError || !t) {
            reject(new Error(chrome.runtime.lastError?.message || 'Not signed in'));
          } else {
            resolve(t);
          }
        });
      });
    } catch {
      // Auth not available — show notification to sign in manually
      chrome.notifications.create(`auth:${email.id}`, {
        type: 'basic',
        iconUrl: 'icons/icon128.png',
        title: 'go-mapi: Sign in required',
        message: `Click to sign in and create draft for: ${email.subject || '(No Subject)'}`,
        priority: 2,
      });
      return; // Keep email in queue
    }

    const draftId = await createDraft(email);

    // Tell host to move JSON to processed
    sendToNativeHost({ type: MSG_TYPE.PROCESS, id: email.id });
    emails.delete(email.id);
    await persistEmails();
    updateBadge();
    broadcastToPopup({ type: 'QUEUE_UPDATE', emails: Array.from(emails.values()) });

    // Upload attachments if any
    if (email.attachments && email.attachments.length > 0) {
      try {
        const token = await getAuthToken();
        await uploadAttachments(draftId, '', token, email.attachments, email.id);
      } catch (uploadError) {
        console.error('[go-mapi] Attachment upload failed:', uploadError);
      }
    }

    // Record draft for popup
    const gmailUrl = `https://mail.google.com/mail/u/0/#drafts?compose=${draftId}`;
    const attachCount = email.attachments?.length || 0;

    recentDrafts.unshift({
      draftId,
      subject: email.subject || '(No Subject)',
      timestamp: new Date().toISOString(),
      attachmentCount: attachCount,
      gmailUrl,
    });
    await persistDrafts();
    updateBadge();
    broadcastToPopup({ type: 'DRAFTS_UPDATE', recentDrafts });

    // Show clickable notification
    const attachText = attachCount > 0 ? ` (${attachCount} attachment${attachCount > 1 ? 's' : ''})` : '';

    chrome.notifications.create(`draft:${draftId}`, {
      type: 'basic',
      iconUrl: 'icons/icon128.png',
      title: 'go-mapi: Draft created',
      message: `${email.subject || '(No Subject)'}${attachText}`,
      priority: 2,
    });
  } catch (error) {
    console.error('[go-mapi] Auto-draft failed:', error);

    // Show error notification
    chrome.notifications.create(`error:${email.id}`, {
      type: 'basic',
      iconUrl: 'icons/icon128.png',
      title: 'go-mapi: Draft failed',
      message: `${email.subject || '(No Subject)'}: ${error}`,
      priority: 2,
    });
  }
}

// Click notification → open Gmail draft or trigger sign-in
chrome.notifications.onClicked.addListener((notificationId) => {
  if (notificationId.startsWith('draft:')) {
    const draftId = notificationId.slice('draft:'.length);
    chrome.tabs.create({
      url: `https://mail.google.com/mail/u/0/#drafts?compose=${draftId}`,
    });
    chrome.notifications.clear(notificationId);
  } else if (notificationId.startsWith('auth:')) {
    // Trigger interactive sign-in, then retry all pending emails
    chrome.identity.getAuthToken({ interactive: true }, (token) => {
      if (token) {
        // Retry all queued emails
        for (const email of emails.values()) {
          autoCreateDraft(email);
        }
      }
    });
    chrome.notifications.clear(notificationId);
  }
});

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
            recentDrafts,
          });
          break;

        case 'clearDrafts':
          recentDrafts = [];
          persistDrafts();
          updateBadge();
          sendResponse({ success: true });
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
