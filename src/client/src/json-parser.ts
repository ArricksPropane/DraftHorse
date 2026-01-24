/**
 * JSON parsing and validation for intercepted MAPI messages
 */

import { MailMessage, Recipient, Attachment, Recipients } from "./mail-queue";
import * as crypto from "crypto";

export interface JsonParseError {
  message: string;
  path?: string;
}

export class JsonParser {
  /**
   * Parse and validate a JSON file from the interceptor
   * Throws if the JSON is invalid or doesn't match the schema
   */
  static parseAndValidate(jsonContent: string): MailMessage {
    let parsed: any;

    // Parse JSON
    try {
      parsed = JSON.parse(jsonContent);
    } catch (error) {
      throw new Error(`Invalid JSON: ${error}`);
    }

    // Validate schema
    const validationError = this.validateSchema(parsed);
    if (validationError) {
      throw new Error(validationError.message);
    }

    // Generate a unique ID for this message
    const id = this.generateMessageId(jsonContent);

    // Convert to MailMessage
    const message: MailMessage = {
      id,
      version: parsed.version,
      timestamp: parsed.timestamp,
      subject: parsed.subject || "",
      body: parsed.body || "",
      bodyFormat: parsed.bodyFormat || "plain",
      recipients: {
        to: parsed.recipients.to || [],
        cc: parsed.recipients.cc || [],
        bcc: parsed.recipients.bcc || [],
      },
      attachments: parsed.attachments || [],
      originApp: parsed.originApp || "unknown",
    };

    return message;
  }

  /**
   * Validate message against the schema
   */
  private static validateSchema(obj: any): JsonParseError | null {
    // Check required top-level fields
    const requiredFields = ["version", "timestamp", "subject", "body", "bodyFormat", "recipients", "attachments", "originApp"];
    for (const field of requiredFields) {
      if (!(field in obj)) {
        return { message: `Missing required field: ${field}` };
      }
    }

    // Validate types
    if (typeof obj.version !== "number") {
      return { message: "version must be a number" };
    }

    if (typeof obj.timestamp !== "string") {
      return { message: "timestamp must be a string" };
    }

    if (typeof obj.subject !== "string") {
      return { message: "subject must be a string" };
    }

    if (typeof obj.body !== "string") {
      return { message: "body must be a string" };
    }

    if (typeof obj.bodyFormat !== "string" || !["plain", "html"].includes(obj.bodyFormat)) {
      return { message: "bodyFormat must be 'plain' or 'html'" };
    }

    // Validate recipients object
    if (typeof obj.recipients !== "object" || obj.recipients === null) {
      return { message: "recipients must be an object" };
    }

    if (!Array.isArray(obj.recipients.to)) {
      return { message: "recipients.to must be an array" };
    }

    if (!Array.isArray(obj.recipients.cc)) {
      return { message: "recipients.cc must be an array" };
    }

    if (!Array.isArray(obj.recipients.bcc)) {
      return { message: "recipients.bcc must be an array" };
    }

    // Validate recipient items
    const validateRecipient = (recip: any): boolean => {
      return (
        typeof recip === "object" &&
        recip !== null &&
        typeof recip.name === "string" &&
        typeof recip.address === "string"
      );
    };

    for (const recip of obj.recipients.to) {
      if (!validateRecipient(recip)) {
        return { message: "Invalid recipient in 'to' array" };
      }
    }

    for (const recip of obj.recipients.cc) {
      if (!validateRecipient(recip)) {
        return { message: "Invalid recipient in 'cc' array" };
      }
    }

    for (const recip of obj.recipients.bcc) {
      if (!validateRecipient(recip)) {
        return { message: "Invalid recipient in 'bcc' array" };
      }
    }

    // Validate attachments array
    if (!Array.isArray(obj.attachments)) {
      return { message: "attachments must be an array" };
    }

    for (const attach of obj.attachments) {
      if (typeof attach !== "object" || attach === null) {
        return { message: "Invalid attachment object" };
      }
      if (typeof attach.filename !== "string") {
        return { message: "Attachment filename must be a string" };
      }
      if (typeof attach.path !== "string") {
        return { message: "Attachment path must be a string" };
      }
      if (typeof attach.size !== "number") {
        return { message: "Attachment size must be a number" };
      }
    }

    if (typeof obj.originApp !== "string") {
      return { message: "originApp must be a string" };
    }

    return null;
  }

  /**
   * Generate a unique ID for a message based on its content
   */
  private static generateMessageId(jsonContent: string): string {
    // Use SHA256 hash of the JSON content + a timestamp
    const hash = crypto.createHash("sha256").update(jsonContent).digest("hex").substring(0, 12);
    const timestamp = Date.now().toString(36);
    return `msg_${timestamp}_${hash}`;
  }
}
