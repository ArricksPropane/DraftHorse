import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import EmailDetail from '../EmailDetail';
import type { EmailWithId } from '../../types/messages';

describe('EmailDetail', () => {
  const mockEmail: EmailWithId = {
    id: 'test-id-1',
    version: 1,
    timestamp: '2024-01-15T10:30:00Z',
    subject: 'Test Email Subject',
    body: 'This is the email body content.',
    bodyFormat: 'plain',
    recipients: {
      to: [{ name: 'John Doe', address: 'john@example.com' }],
      cc: [],
      bcc: [],
    },
    attachments: [],
    originApp: 'Windows Explorer',
  };

  const defaultProps = {
    email: mockEmail,
    onBack: vi.fn(),
    onCreateDraft: vi.fn(),
    onSend: vi.fn(),
    onDelete: vi.fn(),
    sending: false,
  };

  it('should render email subject', () => {
    render(<EmailDetail {...defaultProps} />);

    expect(screen.getByText('Test Email Subject')).toBeInTheDocument();
  });

  it('should display "(No Subject)" for empty subject', () => {
    render(
      <EmailDetail
        {...defaultProps}
        email={{ ...mockEmail, subject: '' }}
      />
    );

    expect(screen.getByText('(No Subject)')).toBeInTheDocument();
  });

  it('should render To recipients', () => {
    render(<EmailDetail {...defaultProps} />);

    expect(screen.getByText(/To:/)).toBeInTheDocument();
    expect(screen.getByText(/John Doe <john@example.com>/)).toBeInTheDocument();
  });

  it('should render CC recipients', () => {
    const emailWithCC: EmailWithId = {
      ...mockEmail,
      recipients: {
        ...mockEmail.recipients,
        cc: [{ name: 'CC Person', address: 'cc@example.com' }],
      },
    };

    render(<EmailDetail {...defaultProps} email={emailWithCC} />);

    expect(screen.getByText(/Cc:/)).toBeInTheDocument();
    expect(screen.getByText(/CC Person <cc@example.com>/)).toBeInTheDocument();
  });

  it('should render BCC recipients', () => {
    const emailWithBCC: EmailWithId = {
      ...mockEmail,
      recipients: {
        ...mockEmail.recipients,
        bcc: [{ name: 'Secret', address: 'bcc@example.com' }],
      },
    };

    render(<EmailDetail {...defaultProps} email={emailWithBCC} />);

    expect(screen.getByText(/Bcc:/)).toBeInTheDocument();
    expect(screen.getByText(/Secret <bcc@example.com>/)).toBeInTheDocument();
  });

  it('should show "(No recipients)" when no recipients', () => {
    const emailNoRecipients: EmailWithId = {
      ...mockEmail,
      recipients: { to: [], cc: [], bcc: [] },
    };

    render(<EmailDetail {...defaultProps} email={emailNoRecipients} />);

    expect(screen.getByText('(No recipients)')).toBeInTheDocument();
  });

  it('should render email body', () => {
    render(<EmailDetail {...defaultProps} />);

    expect(screen.getByText('This is the email body content.')).toBeInTheDocument();
  });

  it('should show "(No content)" for empty body', () => {
    render(
      <EmailDetail
        {...defaultProps}
        email={{ ...mockEmail, body: '' }}
      />
    );

    expect(screen.getByText('(No content)')).toBeInTheDocument();
  });

  it('should render attachments section when present', () => {
    const emailWithAttachments: EmailWithId = {
      ...mockEmail,
      attachments: [
        { filename: 'document.pdf', path: '/tmp/document.pdf', size: 1024 * 1024 },
        { filename: 'image.png', path: '/tmp/image.png', size: 512 * 1024 },
      ],
    };

    render(<EmailDetail {...defaultProps} email={emailWithAttachments} />);

    expect(screen.getByText('Attachments')).toBeInTheDocument();
    expect(screen.getByText(/document.pdf/)).toBeInTheDocument();
    expect(screen.getByText(/1.0 MB/)).toBeInTheDocument();
    expect(screen.getByText(/image.png/)).toBeInTheDocument();
    expect(screen.getByText(/512.0 KB/)).toBeInTheDocument();
  });

  it('should not render attachments section when empty', () => {
    render(<EmailDetail {...defaultProps} />);

    expect(screen.queryByText('Attachments')).not.toBeInTheDocument();
  });

  it('should render origin app and timestamp', () => {
    render(<EmailDetail {...defaultProps} />);

    expect(screen.getByText(/Windows Explorer/)).toBeInTheDocument();
  });

  it('should call onBack when back button is clicked', () => {
    const onBack = vi.fn();
    render(<EmailDetail {...defaultProps} onBack={onBack} />);

    fireEvent.click(screen.getByText('← Back to list'));

    expect(onBack).toHaveBeenCalled();
  });

  it('should call onDelete when Delete button is clicked', () => {
    const onDelete = vi.fn();
    render(<EmailDetail {...defaultProps} onDelete={onDelete} />);

    fireEvent.click(screen.getByText('Delete'));

    expect(onDelete).toHaveBeenCalled();
  });

  it('should call onCreateDraft when Save as Draft button is clicked', () => {
    const onCreateDraft = vi.fn();
    render(<EmailDetail {...defaultProps} onCreateDraft={onCreateDraft} />);

    fireEvent.click(screen.getByText('Save as Draft'));

    expect(onCreateDraft).toHaveBeenCalled();
  });

  it('should call onSend when Send Now button is clicked', () => {
    const onSend = vi.fn();
    render(<EmailDetail {...defaultProps} onSend={onSend} />);

    fireEvent.click(screen.getByText('Send Now'));

    expect(onSend).toHaveBeenCalled();
  });

  it('should disable buttons when sending is true', () => {
    render(<EmailDetail {...defaultProps} sending={true} />);

    expect(screen.getByText('Delete').closest('button')).toBeDisabled();
    // Save as Draft and Send Now are replaced with spinners when sending
  });

  it('should show spinners when sending', () => {
    render(<EmailDetail {...defaultProps} sending={true} />);

    // Spinners replace the button text
    const spinners = document.querySelectorAll('.spinner-border');
    expect(spinners.length).toBeGreaterThan(0);
  });

  it('should format bytes correctly', () => {
    const emailWithSmallAttachment: EmailWithId = {
      ...mockEmail,
      attachments: [{ filename: 'tiny.txt', path: '/tmp/tiny.txt', size: 500 }],
    };

    render(<EmailDetail {...defaultProps} email={emailWithSmallAttachment} />);

    expect(screen.getByText(/500 B/)).toBeInTheDocument();
  });

  it('should format recipient without name correctly', () => {
    const emailNoName: EmailWithId = {
      ...mockEmail,
      recipients: {
        to: [{ name: '', address: 'test@example.com' }],
        cc: [],
        bcc: [],
      },
    };

    render(<EmailDetail {...defaultProps} email={emailNoName} />);

    expect(screen.getByText(/test@example.com/)).toBeInTheDocument();
    // Should not show empty name with angle brackets
    expect(screen.queryByText(/<test@example.com>/)).not.toBeInTheDocument();
  });
});
