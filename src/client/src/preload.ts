/**
 * Preload script - Exposes safe IPC methods to the renderer
 */

import { contextBridge, ipcRenderer } from "electron";
import { MailMessage } from "./mail-queue";

// Define the API that will be available in the renderer
export interface MainApi {
  getQueue: () => Promise<MailMessage[]>;
  sendMail: (id: string, gmailToken: string) => Promise<boolean>;
  deleteMail: (id: string) => Promise<boolean>;
  onQueueUpdated: (callback: (queue: MailMessage[]) => void) => () => void;
  openSettings: () => Promise<void>;
}

const api: MainApi = {
  // Get the current mail queue
  getQueue: () => ipcRenderer.invoke("mail:getQueue"),

  // Send an email via Gmail
  sendMail: (id: string, gmailToken: string) => ipcRenderer.invoke("mail:send", { id, gmailToken }),

  // Delete an email from the queue
  deleteMail: (id: string) => ipcRenderer.invoke("mail:delete", { id }),

  // Subscribe to queue updates
  onQueueUpdated: (callback: (queue: MailMessage[]) => void) => {
    const listener = (_event: any, queue: MailMessage[]) => {
      callback(queue);
    };
    ipcRenderer.on("mail:queueUpdated", listener);
    // Return unsubscribe function
    return () => {
      ipcRenderer.removeListener("mail:queueUpdated", listener);
    };
  },

  // Open settings dialog
  openSettings: () => ipcRenderer.invoke("app:openSettings"),
};

// Expose the API to the renderer
contextBridge.exposeInMainWorld("mainApi", api);
