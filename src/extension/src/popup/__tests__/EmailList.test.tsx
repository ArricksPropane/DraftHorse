import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import EmailList from '../EmailList';
import type { EmailWithId } from '../../types/messages';

describe('EmailList', () => {
  const mockEmail: EmailWithId = {
    id: 'test-id-1',
    version: 1,
    timestamp: new Date().toISOString(),
    subject: 'Test Email Subject',
    body: 'Test body content',
    bodyFormat: 'plain',
    recipients: {
      to: [{ name: 'John Doe', address: 'john@example.com' }],
      cc: [],
      bcc: [],
    },
    attachments: [],
    originApp: 'TestApp',
  };

  it('should render empty state when no emails', () => {
    render(<EmailList emails={[]} onSelect={vi.fn()} />);

    expect(screen.getByText('No pending emails')).toBeInTheDocument();
    expect(screen.getByText(/Use "Send to → Mail recipient"/)).toBeInTheDocument();
  });

  it('should render email list with items', () => {
    render(<EmailList emails={[mockEmail]} onSelect={vi.fn()} />);

    expect(screen.getByText('Test Email Subject')).toBeInTheDocument();
    expect(screen.getByText(/To: John Doe/)).toBeInTheDocument();
  });

  it('should display "(No Subject)" for empty subject', () => {
    const emailNoSubject: EmailWithId = {
      ...mockEmail,
      subject: '',
    };

    render(<EmailList emails={[emailNoSubject]} onSelect={vi.fn()} />);

    expect(screen.getByText('(No Subject)')).toBeInTheDocument();
  });

  it('should call onSelect when email item is clicked', () => {
    const onSelect = vi.fn();
    render(<EmailList emails={[mockEmail]} onSelect={onSelect} />);

    fireEvent.click(screen.getByText('Test Email Subject'));

    expect(onSelect).toHaveBeenCalledWith(mockEmail);
  });

  it('should render multiple emails', () => {
    const emails: EmailWithId[] = [
      mockEmail,
      {
        ...mockEmail,
        id: 'test-id-2',
        subject: 'Second Email',
      },
      {
        ...mockEmail,
        id: 'test-id-3',
        subject: 'Third Email',
      },
    ];

    render(<EmailList emails={emails} onSelect={vi.fn()} />);

    expect(screen.getByText('Test Email Subject')).toBeInTheDocument();
    expect(screen.getByText('Second Email')).toBeInTheDocument();
    expect(screen.getByText('Third Email')).toBeInTheDocument();
  });

  it('should show recipient count for multiple recipients', () => {
    const emailMultipleRecipients: EmailWithId = {
      ...mockEmail,
      recipients: {
        to: [
          { name: 'First', address: 'first@example.com' },
          { name: 'Second', address: 'second@example.com' },
        ],
        cc: [{ name: 'CC', address: 'cc@example.com' }],
        bcc: [],
      },
    };

    render(<EmailList emails={[emailMultipleRecipients]} onSelect={vi.fn()} />);

    expect(screen.getByText(/To: First \+2/)).toBeInTheDocument();
  });

  it('should show "(No recipients)" when no recipients', () => {
    const emailNoRecipients: EmailWithId = {
      ...mockEmail,
      recipients: {
        to: [],
        cc: [],
        bcc: [],
      },
    };

    render(<EmailList emails={[emailNoRecipients]} onSelect={vi.fn()} />);

    expect(screen.getByText('(No recipients)')).toBeInTheDocument();
  });

  it('should show attachment badge when email has attachments', () => {
    const emailWithAttachment: EmailWithId = {
      ...mockEmail,
      attachments: [{ filename: 'file.pdf', path: '/tmp/file.pdf', size: 1024 }],
    };

    render(<EmailList emails={[emailWithAttachment]} onSelect={vi.fn()} />);

    expect(screen.getByText('1')).toBeInTheDocument();
  });

  it('should show attachment count for multiple attachments', () => {
    const emailWithAttachments: EmailWithId = {
      ...mockEmail,
      attachments: [
        { filename: 'file1.pdf', path: '/tmp/file1.pdf', size: 1024 },
        { filename: 'file2.pdf', path: '/tmp/file2.pdf', size: 2048 },
        { filename: 'file3.pdf', path: '/tmp/file3.pdf', size: 4096 },
      ],
    };

    render(<EmailList emails={[emailWithAttachments]} onSelect={vi.fn()} />);

    expect(screen.getByText('3')).toBeInTheDocument();
  });

  it('should use address when recipient has no name', () => {
    const emailNoName: EmailWithId = {
      ...mockEmail,
      recipients: {
        to: [{ name: '', address: 'test@example.com' }],
        cc: [],
        bcc: [],
      },
    };

    render(<EmailList emails={[emailNoName]} onSelect={vi.fn()} />);

    expect(screen.getByText('To: test@example.com')).toBeInTheDocument();
  });

  it('should display "Just now" for recent timestamps', () => {
    const recentEmail: EmailWithId = {
      ...mockEmail,
      timestamp: new Date().toISOString(),
    };

    render(<EmailList emails={[recentEmail]} onSelect={vi.fn()} />);

    expect(screen.getByText('Just now')).toBeInTheDocument();
  });
});
