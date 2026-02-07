import type { MailMessage } from '../types/messages';

/**
 * Builds an RFC 2822 formatted email message from a MailMessage object.
 */
export function buildRfc2822Message(email: MailMessage): string {
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
