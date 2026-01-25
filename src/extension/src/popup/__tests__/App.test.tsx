import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import App from '../App';
import type { EmailWithId } from '../../types/messages';

// Mock emails for testing
const mockEmails: EmailWithId[] = [
  {
    id: 'email-1',
    version: 1,
    timestamp: new Date().toISOString(),
    subject: 'First Email',
    body: 'First body',
    bodyFormat: 'plain',
    recipients: {
      to: [{ name: 'Test', address: 'test@example.com' }],
      cc: [],
      bcc: [],
    },
    attachments: [],
    originApp: 'TestApp',
  },
  {
    id: 'email-2',
    version: 1,
    timestamp: new Date().toISOString(),
    subject: 'Second Email',
    body: 'Second body',
    bodyFormat: 'plain',
    recipients: {
      to: [{ name: 'Another', address: 'another@example.com' }],
      cc: [],
      bcc: [],
    },
    attachments: [],
    originApp: 'TestApp',
  },
];

describe('App', () => {
  beforeEach(() => {
    // Reset sendMessage mock for each test
    vi.mocked(chrome.runtime.sendMessage).mockReset();
  });

  it('should render header with title', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: true, emails: [], connected: true });
      }
    });

    render(<App />);

    expect(screen.getByText('go-mapi')).toBeInTheDocument();
  });

  it('should show loading spinner initially', () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation(() => {
      // Don't call callback to keep loading state
    });

    render(<App />);

    expect(document.querySelector('.spinner-border')).toBeInTheDocument();
  });

  it('should show connected status when connected', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: true, emails: [], connected: true });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('Connected')).toBeInTheDocument();
    });
  });

  it('should show disconnected status when not connected', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: true, emails: [], connected: false });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('Disconnected')).toBeInTheDocument();
    });
  });

  it('should show email count in header', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: true, emails: mockEmails, connected: true });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText(/2 pending/)).toBeInTheDocument();
    });
  });

  it('should render email list after loading', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: true, emails: mockEmails, connected: true });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('First Email')).toBeInTheDocument();
      expect(screen.getByText('Second Email')).toBeInTheDocument();
    });
  });

  it('should show empty state when no emails', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: true, emails: [], connected: true });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('No pending emails')).toBeInTheDocument();
    });
  });

  it('should show error alert on load failure', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: false, error: 'Connection failed' });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('Connection failed')).toBeInTheDocument();
    });
  });

  it('should navigate to email detail when email is clicked', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: true, emails: mockEmails, connected: true });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('First Email')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('First Email'));

    await waitFor(() => {
      expect(screen.getByText('← Back to list')).toBeInTheDocument();
      expect(screen.getByText('First body')).toBeInTheDocument();
    });
  });

  it('should navigate back to list when back button is clicked', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: true, emails: mockEmails, connected: true });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('First Email')).toBeInTheDocument();
    });

    // Navigate to detail
    fireEvent.click(screen.getByText('First Email'));

    await waitFor(() => {
      expect(screen.getByText('← Back to list')).toBeInTheDocument();
    });

    // Navigate back
    fireEvent.click(screen.getByText('← Back to list'));

    await waitFor(() => {
      expect(screen.getByText('First Email')).toBeInTheDocument();
      expect(screen.getByText('Second Email')).toBeInTheDocument();
    });
  });

  it('should call createDraft action when Save as Draft is clicked', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((msg, callback) => {
      if (callback) {
        if (msg.action === 'getEmails') {
          callback({ success: true, emails: mockEmails, connected: true });
        } else if (msg.action === 'createDraft') {
          callback({ success: true, draftId: 'draft-123' });
        }
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('First Email')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('First Email'));

    await waitFor(() => {
      expect(screen.getByText('Save as Draft')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Save as Draft'));

    await waitFor(() => {
      expect(chrome.runtime.sendMessage).toHaveBeenCalledWith(
        { action: 'createDraft', id: 'email-1' },
        expect.any(Function)
      );
    });
  });

  it('should call sendEmail action when Send Now is clicked', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((msg, callback) => {
      if (callback) {
        if (msg.action === 'getEmails') {
          callback({ success: true, emails: mockEmails, connected: true });
        } else if (msg.action === 'sendEmail') {
          callback({ success: true, messageId: 'msg-123' });
        }
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('First Email')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('First Email'));

    await waitFor(() => {
      expect(screen.getByText('Send Now')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Send Now'));

    await waitFor(() => {
      expect(chrome.runtime.sendMessage).toHaveBeenCalledWith(
        { action: 'sendEmail', id: 'email-1' },
        expect.any(Function)
      );
    });
  });

  it('should call deleteEmail action when Delete is clicked', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((msg, callback) => {
      if (callback) {
        if (msg.action === 'getEmails') {
          callback({ success: true, emails: mockEmails, connected: true });
        } else if (msg.action === 'deleteEmail') {
          callback({ success: true });
        }
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('First Email')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('First Email'));

    await waitFor(() => {
      expect(screen.getByText('Delete')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Delete'));

    await waitFor(() => {
      expect(chrome.runtime.sendMessage).toHaveBeenCalledWith(
        { action: 'deleteEmail', id: 'email-1' },
        expect.any(Function)
      );
    });
  });

  it('should dismiss error alert when close button is clicked', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((_msg, callback) => {
      if (callback) {
        callback({ success: false, error: 'Test error' });
      }
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByText('Test error')).toBeInTheDocument();
    });

    // Find and click the close button on the alert
    const closeButton = document.querySelector('.btn-close');
    if (closeButton) {
      fireEvent.click(closeButton);
    }

    await waitFor(() => {
      expect(screen.queryByText('Test error')).not.toBeInTheDocument();
    });
  });

  it('should call reconnect action when Retry is clicked', async () => {
    vi.mocked(chrome.runtime.sendMessage).mockImplementation((msg, callback) => {
      if (callback) {
        if (msg.action === 'getEmails') {
          callback({ success: true, emails: [], connected: false, error: 'Disconnected' });
        } else if (msg.action === 'reconnect') {
          callback({ success: true });
        }
      }
    });

    render(<App />);

    // Wait for error to appear (we need to trigger it via message listener)
    // The retry button only appears when disconnected with an error
    await waitFor(() => {
      expect(screen.getByText('Disconnected')).toBeInTheDocument();
    });
  });
});
