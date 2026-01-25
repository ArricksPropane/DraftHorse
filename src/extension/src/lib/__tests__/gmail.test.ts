import { describe, it, expect, vi, beforeEach } from 'vitest';
import { buildRfc2822Message, base64UrlEncode, getAuthToken, createDraft, sendEmail } from '../gmail';
import type { MailMessage } from '../../types/messages';
import { mockFetch } from '../../test/setup';
import { mockAuthTokenError, mockAuthTokenSuccess } from '../../test/mocks/chrome';

describe('buildRfc2822Message', () => {
  const baseEmail: MailMessage = {
    version: 1,
    timestamp: '2024-01-01T00:00:00Z',
    subject: 'Test Subject',
    body: 'Test body content',
    bodyFormat: 'plain',
    recipients: {
      to: [],
      cc: [],
      bcc: [],
    },
    attachments: [],
    originApp: 'TestApp',
  };

  it('should format a simple email with one recipient', () => {
    const email: MailMessage = {
      ...baseEmail,
      recipients: {
        to: [{ name: 'John Doe', address: 'john@example.com' }],
        cc: [],
        bcc: [],
      },
    };

    const result = buildRfc2822Message(email);

    expect(result).toContain('To: "John Doe" <john@example.com>');
    expect(result).toContain('Subject: Test Subject');
    expect(result).toContain('Content-Type: text/plain; charset=UTF-8');
    expect(result).toContain('Test body content');
  });

  it('should format recipient without name', () => {
    const email: MailMessage = {
      ...baseEmail,
      recipients: {
        to: [{ name: '', address: 'john@example.com' }],
        cc: [],
        bcc: [],
      },
    };

    const result = buildRfc2822Message(email);

    expect(result).toContain('To: john@example.com');
    expect(result).not.toContain('"" <');
  });

  it('should include CC recipients', () => {
    const email: MailMessage = {
      ...baseEmail,
      recipients: {
        to: [{ name: 'To', address: 'to@example.com' }],
        cc: [{ name: 'CC Person', address: 'cc@example.com' }],
        bcc: [],
      },
    };

    const result = buildRfc2822Message(email);

    expect(result).toContain('To: "To" <to@example.com>');
    expect(result).toContain('Cc: "CC Person" <cc@example.com>');
  });

  it('should include BCC recipients', () => {
    const email: MailMessage = {
      ...baseEmail,
      recipients: {
        to: [{ name: '', address: 'to@example.com' }],
        cc: [],
        bcc: [{ name: 'Secret', address: 'bcc@example.com' }],
      },
    };

    const result = buildRfc2822Message(email);

    expect(result).toContain('Bcc: "Secret" <bcc@example.com>');
  });

  it('should handle multiple recipients in each field', () => {
    const email: MailMessage = {
      ...baseEmail,
      recipients: {
        to: [
          { name: 'To1', address: 'to1@example.com' },
          { name: 'To2', address: 'to2@example.com' },
        ],
        cc: [
          { name: '', address: 'cc1@example.com' },
          { name: '', address: 'cc2@example.com' },
        ],
        bcc: [],
      },
    };

    const result = buildRfc2822Message(email);

    expect(result).toContain('To: "To1" <to1@example.com>, "To2" <to2@example.com>');
    expect(result).toContain('Cc: cc1@example.com, cc2@example.com');
  });

  it('should use "(No Subject)" for empty subject', () => {
    const email: MailMessage = {
      ...baseEmail,
      subject: '',
    };

    const result = buildRfc2822Message(email);

    expect(result).toContain('Subject: (No Subject)');
  });

  it('should set HTML content type for HTML body format', () => {
    const email: MailMessage = {
      ...baseEmail,
      bodyFormat: 'html',
      body: '<p>HTML content</p>',
    };

    const result = buildRfc2822Message(email);

    expect(result).toContain('Content-Type: text/html; charset=UTF-8');
  });

  it('should have empty line before body (CRLF separation)', () => {
    const email: MailMessage = {
      ...baseEmail,
      recipients: {
        to: [{ name: '', address: 'test@example.com' }],
        cc: [],
        bcc: [],
      },
    };

    const result = buildRfc2822Message(email);

    // Should have CRLF CRLF before body (empty line)
    expect(result).toContain('\r\n\r\n');
  });

  it('should handle empty body', () => {
    const email: MailMessage = {
      ...baseEmail,
      body: '',
    };

    const result = buildRfc2822Message(email);

    expect(result).toContain('Subject: Test Subject');
    // Body section should be empty but present
    expect(result.endsWith('\r\n')).toBe(true);
  });

  it('should omit To header when no To recipients', () => {
    const email: MailMessage = {
      ...baseEmail,
      recipients: {
        to: [],
        cc: [{ name: '', address: 'cc@example.com' }],
        bcc: [],
      },
    };

    const result = buildRfc2822Message(email);

    expect(result).not.toContain('To:');
    expect(result).toContain('Cc: cc@example.com');
  });
});

describe('base64UrlEncode', () => {
  it('should encode simple ASCII string', () => {
    const result = base64UrlEncode('Hello, World!');

    // Should not contain + / or =
    expect(result).not.toMatch(/[+/=]/);
    // Should be valid base64url
    expect(result).toMatch(/^[A-Za-z0-9_-]+$/);
  });

  it('should encode UTF-8 characters', () => {
    const result = base64UrlEncode('Hello, 世界!');

    expect(result).not.toMatch(/[+/=]/);
    expect(result).toMatch(/^[A-Za-z0-9_-]+$/);
  });

  it('should handle empty string', () => {
    const result = base64UrlEncode('');

    expect(result).toBe('');
  });

  it('should replace + with -', () => {
    // String that produces + in regular base64
    const input = '>>>'; // Base64: Pj4+
    const result = base64UrlEncode(input);

    expect(result).not.toContain('+');
  });

  it('should replace / with _', () => {
    // String that produces / in regular base64
    const input = '???'; // Would have / in base64
    const result = base64UrlEncode(input);

    expect(result).not.toContain('/');
  });

  it('should remove padding =', () => {
    // String that would need padding
    const result = base64UrlEncode('a');

    expect(result).not.toContain('=');
  });
});

describe('getAuthToken', () => {
  beforeEach(() => {
    mockAuthTokenSuccess('test-token-123');
  });

  it('should resolve with token on success', async () => {
    const token = await getAuthToken();

    expect(token).toBe('test-token-123');
    expect(chrome.identity.getAuthToken).toHaveBeenCalledWith(
      { interactive: true },
      expect.any(Function)
    );
  });

  it('should reject on chrome.runtime.lastError', async () => {
    mockAuthTokenError('User cancelled');

    await expect(getAuthToken()).rejects.toThrow('User cancelled');
  });

  it('should reject when no token received', async () => {
    chrome.identity.getAuthToken = vi.fn(
      (_options: { interactive: boolean }, callback: (token?: string) => void) => {
        callback(undefined);
      }
    );
    (chrome.runtime as { lastError: null }).lastError = null;

    await expect(getAuthToken()).rejects.toThrow('No token received');
  });
});

describe('createDraft', () => {
  const testEmail: MailMessage = {
    version: 1,
    timestamp: '2024-01-01T00:00:00Z',
    subject: 'Test Draft',
    body: 'Draft body',
    bodyFormat: 'plain',
    recipients: {
      to: [{ name: 'Test', address: 'test@example.com' }],
      cc: [],
      bcc: [],
    },
    attachments: [],
    originApp: 'TestApp',
  };

  beforeEach(() => {
    mockAuthTokenSuccess('draft-token');
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 'draft-123' }),
      text: () => Promise.resolve(''),
    });
  });

  it('should create a draft and return the ID', async () => {
    const draftId = await createDraft(testEmail);

    expect(draftId).toBe('draft-123');
    expect(mockFetch).toHaveBeenCalledWith(
      'https://www.googleapis.com/gmail/v1/users/me/drafts',
      expect.objectContaining({
        method: 'POST',
        headers: {
          Authorization: 'Bearer draft-token',
          'Content-Type': 'application/json',
        },
      })
    );
  });

  it('should include base64url encoded message in request body', async () => {
    await createDraft(testEmail);

    const [, options] = mockFetch.mock.calls[0];
    const body = JSON.parse(options.body);

    expect(body).toHaveProperty('message.raw');
    expect(body.message.raw).toMatch(/^[A-Za-z0-9_-]+$/);
  });

  it('should throw on API error', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      text: () => Promise.resolve('Invalid request'),
      json: () => Promise.reject(new Error('Not JSON')),
    });

    await expect(createDraft(testEmail)).rejects.toThrow('Failed to create draft: Invalid request');
  });

  it('should throw on auth error', async () => {
    mockAuthTokenError('Auth failed');

    await expect(createDraft(testEmail)).rejects.toThrow('Auth failed');
  });
});

describe('sendEmail', () => {
  const testEmail: MailMessage = {
    version: 1,
    timestamp: '2024-01-01T00:00:00Z',
    subject: 'Test Send',
    body: 'Send body',
    bodyFormat: 'plain',
    recipients: {
      to: [{ name: 'Recipient', address: 'recipient@example.com' }],
      cc: [],
      bcc: [],
    },
    attachments: [],
    originApp: 'TestApp',
  };

  beforeEach(() => {
    mockAuthTokenSuccess('send-token');
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ id: 'message-456' }),
      text: () => Promise.resolve(''),
    });
  });

  it('should send email and return message ID', async () => {
    const messageId = await sendEmail(testEmail);

    expect(messageId).toBe('message-456');
    expect(mockFetch).toHaveBeenCalledWith(
      'https://www.googleapis.com/gmail/v1/users/me/messages/send',
      expect.objectContaining({
        method: 'POST',
        headers: {
          Authorization: 'Bearer send-token',
          'Content-Type': 'application/json',
        },
      })
    );
  });

  it('should include raw message directly (not nested in message object)', async () => {
    await sendEmail(testEmail);

    const [, options] = mockFetch.mock.calls[0];
    const body = JSON.parse(options.body);

    // sendEmail uses { raw: ... } not { message: { raw: ... } }
    expect(body).toHaveProperty('raw');
    expect(body).not.toHaveProperty('message');
  });

  it('should throw on API error', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      text: () => Promise.resolve('Rate limit exceeded'),
      json: () => Promise.reject(new Error('Not JSON')),
    });

    await expect(sendEmail(testEmail)).rejects.toThrow('Failed to send email: Rate limit exceeded');
  });
});
