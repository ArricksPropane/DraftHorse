/**
 * Gmail API integration for sending intercepted emails
 */

import { MailMessage } from "./mail-queue";
import { gmail_v1 } from "@googleapis/gmail";

export class GmailSender {
  private accessToken: string;

  constructor(accessToken: string) {
    this.accessToken = accessToken;
  }

  /**
   * Send a message via Gmail
   */
  async sendMessage(message: MailMessage): Promise<void> {
    try {
      // Create Gmail API client
      const gmail = new gmail_v1.Gmail({
        auth: {
          getAccessToken: async () => {
            return { token: this.accessToken };
          },
        },
      });

      // Build email headers
      const toRecipients = message.recipients.to.map((r) => (r.name ? `${r.name} <${r.address}>` : r.address)).join(", ");
      const ccRecipients =
        message.recipients.cc.length > 0
          ? message.recipients.cc.map((r) => (r.name ? `${r.name} <${r.address}>` : r.address)).join(", ")
          : "";
      const bccRecipients =
        message.recipients.bcc.length > 0
          ? message.recipients.bcc.map((r) => (r.name ? `${r.name} <${r.address}>` : r.address)).join(", ")
          : "";

      let emailContent = `To: ${toRecipients}\r\n`;
      if (ccRecipients) {
        emailContent += `Cc: ${ccRecipients}\r\n`;
      }
      if (bccRecipients) {
        emailContent += `Bcc: ${bccRecipients}\r\n`;
      }
      emailContent += `Subject: ${message.subject}\r\n`;
      emailContent += `Content-Type: ${message.bodyFormat === "html" ? "text/html" : "text/plain"}; charset="UTF-8"\r\n`;
      emailContent += `\r\n${message.body}`;

      // For now, simple send without attachments
      // TODO: Add attachment support using multipart MIME
      const encodedMessage = Buffer.from(emailContent).toString("base64").replace(/\+/g, "-").replace(/\//g, "_").replace(/=/g, "");

      await gmail.users.messages.send({
        userId: "me",
        requestBody: {
          raw: encodedMessage,
        },
      });

      console.log(`Email sent: ${message.subject}`);
    } catch (error) {
      console.error("Failed to send email via Gmail:", error);
      throw error;
    }
  }
}
