// Native Messaging protocol types

export const MSG_TYPE = {
  // Host → Extension
  EMAIL: 'email',
  REMOVED: 'removed',
  READY: 'ready',
  ERROR: 'error',
  UPLOAD_COMPLETE: 'upload-complete',
  UPLOAD_ERROR: 'upload-error',
  UPLOAD_PROGRESS: 'upload-progress',
  // Extension → Host
  PROCESS: 'process',
  DELETE: 'delete',
  LIST: 'list',
  SHUTDOWN: 'shutdown',
  UPLOAD_ATTACHMENTS: 'upload-attachments',
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

export interface NativeUploadCompleteMessage {
  type: typeof MSG_TYPE.UPLOAD_COMPLETE;
  draftId: string;
}

export interface NativeUploadErrorMessage {
  type: typeof MSG_TYPE.UPLOAD_ERROR;
  draftId: string;
  error: string;
}

export interface NativeUploadProgressMessage {
  type: typeof MSG_TYPE.UPLOAD_PROGRESS;
  draftId: string;
  current: number;
  total: number;
  filename: string;
}

export type NativeIncomingMessage =
  | NativeEmailMessage
  | NativeRemovedMessage
  | NativeReadyMessage
  | NativeErrorMessage
  | NativeUploadCompleteMessage
  | NativeUploadErrorMessage
  | NativeUploadProgressMessage;

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

export interface NativeUploadAttachmentsMessage {
  type: typeof MSG_TYPE.UPLOAD_ATTACHMENTS;
  draftId: string;
  messageId: string;
  token: string;
  attachments: { path: string; filename: string }[];
}

export type NativeOutgoingMessage =
  | NativeProcessMessage
  | NativeDeleteMessage
  | NativeListMessage
  | NativeShutdownMessage
  | NativeUploadAttachmentsMessage;

// Internal extension messages (between service worker and popup)
export interface RecentDraft {
  draftId: string;
  subject: string;
  timestamp: string;
  attachmentCount: number;
  gmailUrl: string;
}

export interface ExtensionMessage {
  type: 'QUEUE_UPDATE' | 'CONNECTION_STATUS' | 'ERROR' | 'UPLOAD_PROGRESS' | 'DRAFTS_UPDATE';
  emails?: EmailWithId[];
  connected?: boolean;
  error?: string;
  draftId?: string;
  current?: number;
  total?: number;
  filename?: string;
  recentDrafts?: RecentDraft[];
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
