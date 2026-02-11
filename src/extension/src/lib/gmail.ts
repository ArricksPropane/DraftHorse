import type { MailMessage } from '../types/messages';

/**
 * RFC 2047 MIME-encode a string for use in email headers (Subject, names).
 * Non-ASCII text must be encoded as =?UTF-8?B?<base64>?= to avoid mojibake.
 */
function mimeEncode(text: string): string {
  // Check if encoding is needed (any non-ASCII character)
  if (/^[\x20-\x7E]*$/.test(text)) {
    return text; // Pure ASCII — no encoding needed
  }
  const utf8Bytes = new TextEncoder().encode(text);
  let binary = '';
  for (let i = 0; i < utf8Bytes.length; i++) {
    binary += String.fromCharCode(utf8Bytes[i]);
  }
  return `=?UTF-8?B?${btoa(binary)}?=`;
}

/**
 * Format a recipient for an email header, MIME-encoding the name if needed.
 */
function formatRecipient(r: { name: string; address: string }): string {
  if (r.name) {
    return `${mimeEncode(r.name)} <${r.address}>`;
  }
  return r.address;
}

/**
 * Builds an RFC 2822 formatted email message from a MailMessage object.
 */
export function buildRfc2822Message(email: MailMessage): string {
  const lines: string[] = [];

  // From will be filled by Gmail
  // To
  if (email.recipients.to.length > 0) {
    lines.push(`To: ${email.recipients.to.map(formatRecipient).join(', ')}`);
  }

  // CC
  if (email.recipients.cc.length > 0) {
    lines.push(`Cc: ${email.recipients.cc.map(formatRecipient).join(', ')}`);
  }

  // BCC
  if (email.recipients.bcc.length > 0) {
    lines.push(`Bcc: ${email.recipients.bcc.map(formatRecipient).join(', ')}`);
  }

  // Subject — MIME-encode for non-ASCII
  lines.push(`Subject: ${mimeEncode(email.subject || '(No Subject)')}`);

  // Content-Type
  if (email.bodyFormat === 'html') {
    lines.push('Content-Type: text/html; charset=UTF-8');
  } else {
    lines.push('Content-Type: text/plain; charset=UTF-8');
  }
  lines.push('Content-Transfer-Encoding: base64');

  // Empty line before body
  lines.push('');

  // Body — base64 encode to safely transport UTF-8
  const bodyBytes = new TextEncoder().encode(email.body || '');
  let bodyBinary = '';
  for (let i = 0; i < bodyBytes.length; i++) {
    bodyBinary += String.fromCharCode(bodyBytes[i]);
  }
  lines.push(btoa(bodyBinary));

  return lines.join('\r\n');
}

/**
 * Encodes a string to base64url format (URL-safe base64 without padding).
 */
export function base64UrlEncode(str: string): string {
  const utf8Bytes = new TextEncoder().encode(str);
  let binary = '';
  const chunkSize = 8192;
  for (let i = 0; i < utf8Bytes.length; i += chunkSize) {
    const chunk = utf8Bytes.subarray(i, i + chunkSize);
    binary += String.fromCharCode(...chunk);
  }
  const base64 = btoa(binary);
  return base64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/**
 * Gets an OAuth2 auth token from Chrome identity API.
 */
export async function getAuthToken(): Promise<string> {
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

/**
 * Creates a Gmail draft from a MailMessage.
 */
export async function createDraft(email: MailMessage): Promise<string> {
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

/**
 * Sends an email via Gmail API.
 */
export async function sendEmail(email: MailMessage): Promise<string> {
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
