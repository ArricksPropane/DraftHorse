import { Button, Spinner } from 'react-bootstrap';
import type { EmailWithId } from '../types/messages';

interface EmailDetailProps {
  email: EmailWithId;
  onBack: () => void;
  onCreateDraft: () => void;
  onDelete: () => void;
  sending: boolean;
}

export default function EmailDetail({
  email,
  onBack,
  onCreateDraft,
  onDelete,
  sending,
}: EmailDetailProps) {
  const allRecipients = [
    ...email.recipients.to.map((r) => ({ ...r, type: 'To' })),
    ...email.recipients.cc.map((r) => ({ ...r, type: 'Cc' })),
    ...email.recipients.bcc.map((r) => ({ ...r, type: 'Bcc' })),
  ];

  return (
    <>
      <div className="email-detail">
        <button
          className="btn btn-link p-0 mb-2"
          onClick={onBack}
          style={{ fontSize: '0.85rem' }}
        >
          ← Back to list
        </button>

        <h2>{email.subject || '(No Subject)'}</h2>

        <div className="field-label">Recipients</div>
        <div className="field-value">
          {allRecipients.length > 0 ? (
            allRecipients.map((r, i) => (
              <div key={i} style={{ fontSize: '0.8rem' }}>
                <strong>{r.type}:</strong>{' '}
                {r.name ? `${r.name} <${r.address}>` : r.address}
              </div>
            ))
          ) : (
            <span style={{ color: '#6c757d' }}>(No recipients)</span>
          )}
        </div>

        {email.attachments.length > 0 && (
          <>
            <div className="field-label">Attachments</div>
            <div className="field-value">
              {email.attachments.map((att, i) => (
                <div key={i} style={{ fontSize: '0.8rem' }}>
                  {att.filename} ({formatFileSize(att.size)})
                </div>
              ))}
            </div>
          </>
        )}

        <div className="field-label">Body</div>
        <div className="body-preview">
          {email.body || '(No content)'}
        </div>

        <div className="field-label mt-2">Source</div>
        <div className="field-value" style={{ fontSize: '0.75rem', color: '#6c757d' }}>
          {email.originApp} • {formatTimestamp(email.timestamp)}
        </div>
      </div>

      <div className="action-buttons">
        <Button
          variant="outline-danger"
          onClick={onDelete}
          disabled={sending}
          size="sm"
        >
          Delete
        </Button>
        <Button
          variant="primary"
          onClick={onCreateDraft}
          disabled={sending}
          size="sm"
        >
          {sending ? (
            <Spinner animation="border" size="sm" />
          ) : (
            'Save as Draft'
          )}
        </Button>
      </div>
    </>
  );
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function formatTimestamp(timestamp: string): string {
  try {
    return new Date(timestamp).toLocaleString();
  } catch {
    return timestamp;
  }
}
