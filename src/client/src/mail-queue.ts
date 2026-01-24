/**
 * Mail queue management
 * Stores and manages pending emails captured from MAPI
 */

export interface Recipient {
  name: string;
  address: string;
}

export interface Attachment {
  filename: string;
  path: string;
  size: number;
}

export interface Recipients {
  to: Recipient[];
  cc: Recipient[];
  bcc: Recipient[];
}

export interface MailMessage {
  id: string;  // Generated unique ID
  version: number;
  timestamp: string;
  subject: string;
  body: string;
  bodyFormat: "plain" | "html";
  recipients: Recipients;
  attachments: Attachment[];
  originApp: string;
}

export class MailQueue {
  private queue: Map<string, MailMessage> = new Map();
  private listeners: Set<(queue: MailMessage[]) => void> = new Set();

  /**
   * Add an email to the queue
   */
  add(message: MailMessage): void {
    this.queue.set(message.id, message);
    this.notifyListeners();
  }

  /**
   * Remove an email from the queue by ID
   */
  remove(id: string): boolean {
    const result = this.queue.delete(id);
    if (result) {
      this.notifyListeners();
    }
    return result;
  }

  /**
   * Get all emails in the queue
   */
  getAll(): MailMessage[] {
    return Array.from(this.queue.values());
  }

  /**
   * Get an email by ID
   */
  getById(id: string): MailMessage | undefined {
    return this.queue.get(id);
  }

  /**
   * Clear the entire queue
   */
  clear(): void {
    this.queue.clear();
    this.notifyListeners();
  }

  /**
   * Get the number of messages in the queue
   */
  size(): number {
    return this.queue.size;
  }

  /**
   * Subscribe to queue changes
   */
  subscribe(listener: (queue: MailMessage[]) => void): () => void {
    this.listeners.add(listener);
    // Return unsubscribe function
    return () => {
      this.listeners.delete(listener);
    };
  }

  /**
   * Notify all listeners of changes
   */
  private notifyListeners(): void {
    const currentQueue = this.getAll();
    this.listeners.forEach(listener => {
      try {
        listener(currentQueue);
      } catch (error) {
        console.error("Error in mail queue listener:", error);
      }
    });
  }
}
