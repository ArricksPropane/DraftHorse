// Native Messaging protocol types

export const MSG_TYPE = {
  // Host → Extension
  EMAIL: 'email',
  REMOVED: 'removed',
  READY: 'ready',
  ERROR: 'error',
  // Extension → Host
  PROCESS: 'process',
  DELETE: 'delete',
  LIST: 'list',
  SHUTDOWN: 'shutdown',
} as const;

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
  version: number;
  timestamp: string;
  subject: string;
  body: string;
  bodyFormat: 'plain' | 'html';
  recipients: Recipients;
  attachments: Attachment[];
  originApp: string;
}

export interface EmailWithId extends MailMessage {
  id: string;
}

// Messages from native host
export interface NativeEmailMessage {
  type: typeof MSG_TYPE.EMAIL;
  id: string;
  data: MailMessage;
}

export interface NativeRemovedMessage {
  type: typeof MSG_TYPE.REMOVED;
  id: string;
}

export interface NativeReadyMessage {
  type: typeof MSG_TYPE.READY;
  version: string;
}

export interface NativeErrorMessage {
  type: typeof MSG_TYPE.ERROR;
  error: string;
}

export type NativeIncomingMessage =
  | NativeEmailMessage
  | NativeRemovedMessage
  | NativeReadyMessage
  | NativeErrorMessage;

// Messages to native host
export interface NativeProcessMessage {
  type: typeof MSG_TYPE.PROCESS;
  id: string;
}

export interface NativeDeleteMessage {
  type: typeof MSG_TYPE.DELETE;
  id: string;
}

export interface NativeListMessage {
  type: typeof MSG_TYPE.LIST;
}

export interface NativeShutdownMessage {
  type: typeof MSG_TYPE.SHUTDOWN;
}

export type NativeOutgoingMessage =
  | NativeProcessMessage
  | NativeDeleteMessage
  | NativeListMessage
  | NativeShutdownMessage;

// Internal extension messages (between service worker and popup)
export interface ExtensionMessage {
  type: 'QUEUE_UPDATE' | 'CONNECTION_STATUS' | 'ERROR';
  emails?: EmailWithId[];
  connected?: boolean;
  error?: string;
}

// User settings
export interface Settings {
  defaultAction: 'draft' | 'send';
  showNotifications: boolean;
}

export const DEFAULT_SETTINGS: Settings = {
  defaultAction: 'draft',
  showNotifications: true,
};
