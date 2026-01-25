import { describe, it, expect } from 'vitest';
import { readFileSync } from 'fs';
import { join } from 'path';
import type {
  NativeIncomingMessage,
  NativeOutgoingMessage,
} from '../types/messages';
import { MSG_TYPE } from '../types/messages';

/**
 * Protocol Integration Tests
 *
 * These tests validate that the TypeScript implementation correctly parses
 * and generates messages matching the shared protocol fixtures.
 * This ensures compatibility between Go and TypeScript implementations.
 */

function loadFixture(name: string): string {
  // Navigate from src/extension/src/test to repo root, then into tests/protocol-fixtures
  const path = join(__dirname, '..', '..', '..', '..', 'tests', 'protocol-fixtures', name);
  return readFileSync(path, 'utf-8');
}

describe('Protocol Fixtures - Incoming Messages (Host → Extension)', () => {
  it('should parse ready-message.json', () => {
    const fixture = loadFixture('ready-message.json');
    const msg: NativeIncomingMessage = JSON.parse(fixture);

    expect(msg.type).toBe(MSG_TYPE.READY);
    if (msg.type === MSG_TYPE.READY) {
      expect(msg.version).toBe('1.0.0');
    }
  });

  it('should parse email-message.json', () => {
    const fixture = loadFixture('email-message.json');
    const msg: NativeIncomingMessage = JSON.parse(fixture);

    expect(msg.type).toBe(MSG_TYPE.EMAIL);
    if (msg.type === MSG_TYPE.EMAIL) {
      expect(msg.id).toBe('abc123def456');
      expect(msg.data.subject).toBe('Test Email Subject');
      expect(msg.data.body).toBe('This is the email body content.');
      expect(msg.data.bodyFormat).toBe('plain');
      expect(msg.data.recipients.to).toHaveLength(1);
      expect(msg.data.recipients.to[0].name).toBe('John Doe');
      expect(msg.data.recipients.to[0].address).toBe('john@example.com');
      expect(msg.data.recipients.cc).toHaveLength(1);
      expect(msg.data.recipients.bcc).toHaveLength(0);
      expect(msg.data.attachments).toHaveLength(1);
      expect(msg.data.attachments[0].filename).toBe('document.pdf');
      expect(msg.data.originApp).toBe('Windows Explorer');
    }
  });

  it('should parse removed-message.json', () => {
    const fixture = loadFixture('removed-message.json');
    const msg: NativeIncomingMessage = JSON.parse(fixture);

    expect(msg.type).toBe(MSG_TYPE.REMOVED);
    if (msg.type === MSG_TYPE.REMOVED) {
      expect(msg.id).toBe('abc123def456');
    }
  });

  it('should parse error-message.json', () => {
    const fixture = loadFixture('error-message.json');
    const msg: NativeIncomingMessage = JSON.parse(fixture);

    expect(msg.type).toBe(MSG_TYPE.ERROR);
    if (msg.type === MSG_TYPE.ERROR) {
      expect(msg.error).toBe('Failed to process email: invalid JSON format');
    }
  });
});

describe('Protocol Fixtures - Outgoing Messages (Extension → Host)', () => {
  it('should parse list-command.json', () => {
    const fixture = loadFixture('list-command.json');
    const msg: NativeOutgoingMessage = JSON.parse(fixture);

    expect(msg.type).toBe(MSG_TYPE.LIST);
  });

  it('should parse process-command.json', () => {
    const fixture = loadFixture('process-command.json');
    const msg: NativeOutgoingMessage = JSON.parse(fixture);

    expect(msg.type).toBe(MSG_TYPE.PROCESS);
    if (msg.type === MSG_TYPE.PROCESS) {
      expect(msg.id).toBe('abc123def456');
    }
  });

  it('should parse delete-command.json', () => {
    const fixture = loadFixture('delete-command.json');
    const msg: NativeOutgoingMessage = JSON.parse(fixture);

    expect(msg.type).toBe(MSG_TYPE.DELETE);
    if (msg.type === MSG_TYPE.DELETE) {
      expect(msg.id).toBe('abc123def456');
    }
  });

  it('should parse shutdown-command.json', () => {
    const fixture = loadFixture('shutdown-command.json');
    const msg: NativeOutgoingMessage = JSON.parse(fixture);

    expect(msg.type).toBe(MSG_TYPE.SHUTDOWN);
  });
});

describe('Protocol Message Generation', () => {
  it('should generate valid list command', () => {
    const msg: NativeOutgoingMessage = { type: MSG_TYPE.LIST };
    const json = JSON.stringify(msg);
    const parsed = JSON.parse(json);

    expect(parsed.type).toBe('list');
  });

  it('should generate valid process command', () => {
    const msg: NativeOutgoingMessage = { type: MSG_TYPE.PROCESS, id: 'test-id' };
    const json = JSON.stringify(msg);
    const parsed = JSON.parse(json);

    expect(parsed.type).toBe('process');
    expect(parsed.id).toBe('test-id');
  });

  it('should generate valid delete command', () => {
    const msg: NativeOutgoingMessage = { type: MSG_TYPE.DELETE, id: 'test-id' };
    const json = JSON.stringify(msg);
    const parsed = JSON.parse(json);

    expect(parsed.type).toBe('delete');
    expect(parsed.id).toBe('test-id');
  });

  it('should generate valid shutdown command', () => {
    const msg: NativeOutgoingMessage = { type: MSG_TYPE.SHUTDOWN };
    const json = JSON.stringify(msg);
    const parsed = JSON.parse(json);

    expect(parsed.type).toBe('shutdown');
  });
});

describe('Protocol Type Validation', () => {
  it('should have consistent MSG_TYPE constants', () => {
    // Verify the constants match the expected protocol values
    expect(MSG_TYPE.EMAIL).toBe('email');
    expect(MSG_TYPE.REMOVED).toBe('removed');
    expect(MSG_TYPE.READY).toBe('ready');
    expect(MSG_TYPE.ERROR).toBe('error');
    expect(MSG_TYPE.PROCESS).toBe('process');
    expect(MSG_TYPE.DELETE).toBe('delete');
    expect(MSG_TYPE.LIST).toBe('list');
    expect(MSG_TYPE.SHUTDOWN).toBe('shutdown');
  });
});
