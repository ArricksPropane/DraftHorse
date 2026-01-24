/**
 * Renderer process - UI logic
 */

import { MailMessage } from "../mail-queue";

// Type definitions for the exposed API
declare global {
  interface Window {
    mainApi: {
      getQueue: () => Promise<MailMessage[]>;
      sendMail: (id: string, gmailToken: string) => Promise<boolean>;
      deleteMail: (id: string) => Promise<boolean>;
      onQueueUpdated: (callback: (queue: MailMessage[]) => void) => () => void;
      openSettings: () => Promise<void>;
    };
  }
}

class GoMapiClient {
  private currentMessage: MailMessage | null = null;
  private unsubscribeQueue: (() => void) | null = null;

  async init(): Promise<void> {
    this.setupEventListeners();
    await this.loadQueue();

    // Subscribe to queue updates
    this.unsubscribeQueue = window.mainApi.onQueueUpdated((queue) => {
      this.renderQueue(queue);
    });
  }

  /**
   * Set up UI event listeners
   */
  private setupEventListeners(): void {
    document.getElementById("settings-btn")?.addEventListener("click", () => {
      window.mainApi.openSettings();
    });

    document.getElementById("refresh-btn")?.addEventListener("click", () => {
      this.loadQueue();
    });

    document.getElementById("close-details")?.addEventListener("click", () => {
      this.closeDetailsPanel();
    });

    document.getElementById("send-btn")?.addEventListener("click", () => {
      this.sendCurrentMessage();
    });

    document.getElementById("delete-btn")?.addEventListener("click", () => {
      this.deleteCurrentMessage();
    });
  }

  /**
   * Load and display the mail queue
   */
  private async loadQueue(): Promise<void> {
    try {
      const queue = await window.mainApi.getQueue();
      this.renderQueue(queue);
    } catch (error) {
      console.error("Failed to load queue:", error);
    }
  }

  /**
   * Render the mail queue
   */
  private renderQueue(queue: MailMessage[]): void {
    const emptyState = document.getElementById("empty-state");
    const queueList = document.getElementById("queue-list");

    if (queue.length === 0) {
      if (emptyState) emptyState.style.display = "flex";
      if (queueList) queueList.style.display = "none";
      return;
    }

    if (emptyState) emptyState.style.display = "none";
    if (queueList) {
      queueList.style.display = "block";
      queueList.innerHTML = queue.map((msg) => this.createMailItem(msg)).join("");

      // Add click listeners to mail items
      queue.forEach((msg) => {
        const item = document.querySelector(`[data-message-id="${msg.id}"]`);
        if (item) {
          item.addEventListener("click", () => {
            this.showDetailsPanel(msg);
          });
        }
      });
    }
  }

  /**
   * Create HTML for a mail item
   */
  private createMailItem(msg: MailMessage): string {
    const timestamp = new Date(msg.timestamp).toLocaleString();
    const recipients = msg.recipients.to.map((r) => r.name || r.address).join(", ");

    return `
      <div class="mail-item" data-message-id="${msg.id}">
        <div class="mail-item-header">
          <h3 class="mail-subject">${this.escapeHtml(msg.subject || "(no subject)")}</h3>
          <span class="mail-timestamp">${timestamp}</span>
        </div>
        <div class="mail-item-preview">
          <p class="mail-recipients"><strong>To:</strong> ${this.escapeHtml(recipients)}</p>
          <p class="mail-origin"><small>From: ${this.escapeHtml(msg.originApp)}</small></p>
        </div>
      </div>
    `;
  }

  /**
   * Show the details panel for a message
   */
  private showDetailsPanel(msg: MailMessage): void {
    this.currentMessage = msg;

    const detailsPanel = document.getElementById("details-panel");
    if (detailsPanel) {
      detailsPanel.style.display = "block";
    }

    // Populate details
    const setElementText = (id: string, text: string) => {
      const el = document.getElementById(id);
      if (el) el.textContent = text;
    };

    const setElementHtml = (id: string, html: string) => {
      const el = document.getElementById(id);
      if (el) el.innerHTML = html;
    };

    setElementText("detail-from", msg.originApp);
    setElementText("detail-to", msg.recipients.to.map((r) => `${r.name} <${r.address}>`).join(", "));
    setElementText("detail-subject", msg.subject);

    // Set CC/BCC if present
    const ccGroup = document.getElementById("cc-group");
    const bccGroup = document.getElementById("bcc-group");

    if (msg.recipients.cc.length > 0) {
      if (ccGroup) ccGroup.style.display = "block";
      setElementText("detail-cc", msg.recipients.cc.map((r) => `${r.name} <${r.address}>`).join(", "));
    } else if (ccGroup) {
      ccGroup.style.display = "none";
    }

    if (msg.recipients.bcc.length > 0) {
      if (bccGroup) bccGroup.style.display = "block";
      setElementText("detail-bcc", msg.recipients.bcc.map((r) => `${r.name} <${r.address}>`).join(", "));
    } else if (bccGroup) {
      bccGroup.style.display = "none";
    }

    // Set body
    if (msg.bodyFormat === "html") {
      setElementHtml("detail-body", msg.body);
    } else {
      setElementText("detail-body", msg.body);
    }

    // Set attachments
    const attachmentsGroup = document.getElementById("attachments-group");
    const attachmentsList = document.getElementById("detail-attachments");

    if (msg.attachments.length > 0) {
      if (attachmentsGroup) attachmentsGroup.style.display = "block";
      if (attachmentsList) {
        attachmentsList.innerHTML = msg.attachments
          .map(
            (a) =>
              `<li><span class="attachment-icon">📎</span> ${this.escapeHtml(a.filename)} (${this.formatFileSize(a.size)})</li>`
          )
          .join("");
      }
    } else if (attachmentsGroup) {
      attachmentsGroup.style.display = "none";
    }

    setElementText("detail-origin", msg.originApp);
  }

  /**
   * Close the details panel
   */
  private closeDetailsPanel(): void {
    const detailsPanel = document.getElementById("details-panel");
    if (detailsPanel) {
      detailsPanel.style.display = "none";
    }
    this.currentMessage = null;
  }

  /**
   * Send the current message
   */
  private async sendCurrentMessage(): Promise<void> {
    if (!this.currentMessage) {
      return;
    }

    // For now, we'll prompt for a Gmail token
    // In a real app, this would be handled through OAuth flow
    const gmailToken = prompt("Enter your Gmail access token:");
    if (!gmailToken) {
      return;
    }

    try {
      const success = await window.mainApi.sendMail(this.currentMessage.id, gmailToken);
      if (success) {
        alert("Email sent successfully!");
        this.closeDetailsPanel();
        this.loadQueue();
      } else {
        alert("Failed to send email");
      }
    } catch (error) {
      console.error("Send error:", error);
      alert("Error sending email");
    }
  }

  /**
   * Delete the current message
   */
  private async deleteCurrentMessage(): Promise<void> {
    if (!this.currentMessage) {
      return;
    }

    if (!confirm("Are you sure you want to delete this email?")) {
      return;
    }

    try {
      await window.mainApi.deleteMail(this.currentMessage.id);
      this.closeDetailsPanel();
      this.loadQueue();
    } catch (error) {
      console.error("Delete error:", error);
      alert("Error deleting email");
    }
  }

  /**
   * Escape HTML special characters
   */
  private escapeHtml(text: string): string {
    const div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }

  /**
   * Format file size for display
   */
  private formatFileSize(bytes: number): string {
    if (bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
  }
}

// Initialize the client when DOM is ready
document.addEventListener("DOMContentLoaded", () => {
  const client = new GoMapiClient();
  client.init();
});
