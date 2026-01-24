/**
 * Unit tests for JsonParser
 */

import { JsonParser } from "../json-parser";

describe("JsonParser", () => {
  describe("parseAndValidate", () => {
    it("should parse a valid JSON message", () => {
      const jsonContent = `{
        "version": 1,
        "timestamp": "2024-01-15T10:30:00.000Z",
        "subject": "Test Email",
        "body": "Test body",
        "bodyFormat": "plain",
        "recipients": {
          "to": [{"name": "John", "address": "john@example.com"}],
          "cc": [],
          "bcc": []
        },
        "attachments": [],
        "originApp": "test.exe"
      }`;

      const message = JsonParser.parseAndValidate(jsonContent);

      expect(message.subject).toBe("Test Email");
      expect(message.body).toBe("Test body");
      expect(message.recipients.to).toHaveLength(1);
      expect(message.recipients.to[0].name).toBe("John");
    });

    it("should throw on invalid JSON", () => {
      const invalidJson = "{invalid json}";

      expect(() => {
        JsonParser.parseAndValidate(invalidJson);
      }).toThrow();
    });

    it("should throw when required fields are missing", () => {
      const jsonMissingSubject = `{
        "version": 1,
        "timestamp": "2024-01-15T10:30:00.000Z",
        "body": "Test",
        "bodyFormat": "plain",
        "recipients": {"to": [], "cc": [], "bcc": []},
        "attachments": [],
        "originApp": "test.exe"
      }`;

      expect(() => {
        JsonParser.parseAndValidate(jsonMissingSubject);
      }).toThrow();
    });

    it("should throw on invalid bodyFormat", () => {
      const jsonInvalidFormat = `{
        "version": 1,
        "timestamp": "2024-01-15T10:30:00.000Z",
        "subject": "Test",
        "body": "Test",
        "bodyFormat": "invalid",
        "recipients": {"to": [], "cc": [], "bcc": []},
        "attachments": [],
        "originApp": "test.exe"
      }`;

      expect(() => {
        JsonParser.parseAndValidate(jsonInvalidFormat);
      }).toThrow();
    });

    it("should throw on non-array recipients", () => {
      const jsonInvalidRecipients = `{
        "version": 1,
        "timestamp": "2024-01-15T10:30:00.000Z",
        "subject": "Test",
        "body": "Test",
        "bodyFormat": "plain",
        "recipients": {"to": "not an array", "cc": [], "bcc": []},
        "attachments": [],
        "originApp": "test.exe"
      }`;

      expect(() => {
        JsonParser.parseAndValidate(jsonInvalidRecipients);
      }).toThrow();
    });

    it("should handle multiple recipients", () => {
      const jsonContent = `{
        "version": 1,
        "timestamp": "2024-01-15T10:30:00.000Z",
        "subject": "Test",
        "body": "Test",
        "bodyFormat": "plain",
        "recipients": {
          "to": [
            {"name": "John", "address": "john@example.com"},
            {"name": "Jane", "address": "jane@example.com"}
          ],
          "cc": [{"name": "Manager", "address": "mgr@example.com"}],
          "bcc": []
        },
        "attachments": [],
        "originApp": "test.exe"
      }`;

      const message = JsonParser.parseAndValidate(jsonContent);

      expect(message.recipients.to).toHaveLength(2);
      expect(message.recipients.cc).toHaveLength(1);
      expect(message.recipients.bcc).toHaveLength(0);
    });

    it("should handle attachments", () => {
      const jsonContent = `{
        "version": 1,
        "timestamp": "2024-01-15T10:30:00.000Z",
        "subject": "Test",
        "body": "Test",
        "bodyFormat": "plain",
        "recipients": {"to": [], "cc": [], "bcc": []},
        "attachments": [
          {"filename": "file.txt", "path": "C:\\\\file.txt", "size": 1024},
          {"filename": "image.png", "path": "C:\\\\image.png", "size": 2048}
        ],
        "originApp": "test.exe"
      }`;

      const message = JsonParser.parseAndValidate(jsonContent);

      expect(message.attachments).toHaveLength(2);
      expect(message.attachments[0].filename).toBe("file.txt");
      expect(message.attachments[0].size).toBe(1024);
    });

    it("should generate unique IDs", () => {
      const jsonContent = `{
        "version": 1,
        "timestamp": "2024-01-15T10:30:00.000Z",
        "subject": "Test",
        "body": "Test",
        "bodyFormat": "plain",
        "recipients": {"to": [], "cc": [], "bcc": []},
        "attachments": [],
        "originApp": "test.exe"
      }`;

      const msg1 = JsonParser.parseAndValidate(jsonContent);
      const msg2 = JsonParser.parseAndValidate(jsonContent);

      // Same content should generate same ID (deterministic)
      expect(msg1.id).toBe(msg2.id);

      // Different content should generate different ID
      const differentJson = `{
        "version": 1,
        "timestamp": "2024-01-15T10:30:00.000Z",
        "subject": "Different",
        "body": "Test",
        "bodyFormat": "plain",
        "recipients": {"to": [], "cc": [], "bcc": []},
        "attachments": [],
        "originApp": "test.exe"
      }`;

      const msg3 = JsonParser.parseAndValidate(differentJson);
      expect(msg3.id).not.toBe(msg1.id);
    });
  });
});
