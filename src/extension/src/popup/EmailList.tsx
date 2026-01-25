import React from 'react';
import type { EmailWithId } from '../types/messages';

interface EmailListProps {
  emails: EmailWithId[];
  onSelect: (email: EmailWithId) => void;
}

export default function EmailList({ emails, onSelect }: EmailListProps) {
  if (emails.length === 0) {
    return (
      <div className="empty-state">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="48"
          height="48"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.5"
        >
          <path d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
        </svg>
        <h3>No pending emails</h3>
        <p>
          Use "Send to → Mail recipient" in Windows Explorer
          <br />
          to queue emails for Gmail
        </p>
      </div>
    );
  }

  return (
    <ul className="email-list">
      {emails.map((email) => (
        <li
          key={email.id}
          className="email-item"
          onClick={() => onSelect(email)}
        >
          <div className="email-subject">
            {email.subject || '(No Subject)'}
          </div>
          <div className="email-recipients">
            {formatRecipients(email)}
          </div>
          <div className="email-meta">
            <span>{formatTime(email.timestamp)}</span>
            {email.attachments.length > 0 && (
              <span className="attachment-badge">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="12"
                  height="12"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <path d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48" />
                </svg>
                {email.attachments.length}
              </span>
            )}
          </div>
        </li>
      ))}
    </ul>
  );
}

function formatRecipients(email: EmailWithId): string {
  const all = [
    ...email.recipients.to,
    ...email.recipients.cc,
    ...email.recipients.bcc,
  ];

  if (all.length === 0) {
    return '(No recipients)';
  }

  const first = all[0];
  const name = first.name || first.address;

  if (all.length === 1) {
    return `To: ${name}`;
  }

  return `To: ${name} +${all.length - 1}`;
}

function formatTime(timestamp: string): string {
  try {
    const date = new Date(timestamp);
    const now = new Date();
    const diff = now.getTime() - date.getTime();

    // Less than 1 minute
    if (diff < 60000) {
      return 'Just now';
    }

    // Less than 1 hour
    if (diff < 3600000) {
      const mins = Math.floor(diff / 60000);
      return `${mins}m ago`;
    }

    // Less than 24 hours
    if (diff < 86400000) {
      const hours = Math.floor(diff / 3600000);
      return `${hours}h ago`;
    }

    // Format as date
    return date.toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
    });
  } catch {
    return '';
  }
}
