import React, { useState, useEffect, useCallback } from 'react';
import { Alert, Spinner } from 'react-bootstrap';
import EmailList from './EmailList';
import EmailDetail from './EmailDetail';
import type { EmailWithId, ExtensionMessage } from '../types/messages';

interface AppState {
  emails: EmailWithId[];
  selectedEmail: EmailWithId | null;
  connected: boolean;
  loading: boolean;
  error: string | null;
  sending: boolean;
}

export default function App() {
  const [state, setState] = useState<AppState>({
    emails: [],
    selectedEmail: null,
    connected: false,
    loading: true,
    error: null,
    sending: false,
  });

  // Load initial data
  useEffect(() => {
    chrome.runtime.sendMessage({ action: 'getEmails' }, (response) => {
      if (response?.success) {
        setState((s) => ({
          ...s,
          emails: response.emails || [],
          connected: response.connected || false,
          loading: false,
        }));
      } else {
        setState((s) => ({
          ...s,
          loading: false,
          error: response?.error || 'Failed to load emails',
        }));
      }
    });
  }, []);

  // Listen for updates from service worker
  useEffect(() => {
    const listener = (message: ExtensionMessage) => {
      switch (message.type) {
        case 'QUEUE_UPDATE':
          setState((s) => {
            const newState = { ...s, emails: message.emails || [] };
            // Clear selection if selected email was removed
            if (s.selectedEmail && !message.emails?.find((e) => e.id === s.selectedEmail?.id)) {
              newState.selectedEmail = null;
            }
            return newState;
          });
          break;
        case 'CONNECTION_STATUS':
          setState((s) => ({
            ...s,
            connected: message.connected || false,
            error: message.error || null,
          }));
          break;
        case 'ERROR':
          setState((s) => ({ ...s, error: message.error || null }));
          break;
      }
    };

    chrome.runtime.onMessage.addListener(listener);
    return () => chrome.runtime.onMessage.removeListener(listener);
  }, []);

  const handleSelect = useCallback((email: EmailWithId) => {
    setState((s) => ({ ...s, selectedEmail: email }));
  }, []);

  const handleBack = useCallback(() => {
    setState((s) => ({ ...s, selectedEmail: null }));
  }, []);

  const handleCreateDraft = useCallback(async () => {
    if (!state.selectedEmail) return;

    setState((s) => ({ ...s, sending: true, error: null }));

    chrome.runtime.sendMessage(
      { action: 'createDraft', id: state.selectedEmail.id },
      (response) => {
        setState((s) => ({ ...s, sending: false }));
        if (!response?.success) {
          setState((s) => ({ ...s, error: response?.error || 'Failed to create draft' }));
        }
      }
    );
  }, [state.selectedEmail]);

  const handleSend = useCallback(async () => {
    if (!state.selectedEmail) return;

    setState((s) => ({ ...s, sending: true, error: null }));

    chrome.runtime.sendMessage(
      { action: 'sendEmail', id: state.selectedEmail.id },
      (response) => {
        setState((s) => ({ ...s, sending: false }));
        if (response?.success) {
          // Show success briefly
          setState((s) => ({ ...s, selectedEmail: null }));
        } else {
          setState((s) => ({ ...s, error: response?.error || 'Failed to send email' }));
        }
      }
    );
  }, [state.selectedEmail]);

  const handleDelete = useCallback(async () => {
    if (!state.selectedEmail) return;

    chrome.runtime.sendMessage(
      { action: 'deleteEmail', id: state.selectedEmail.id },
      (response) => {
        if (!response?.success) {
          setState((s) => ({ ...s, error: response?.error || 'Failed to delete email' }));
        } else {
          setState((s) => ({ ...s, selectedEmail: null }));
        }
      }
    );
  }, [state.selectedEmail]);

  const handleReconnect = useCallback(() => {
    setState((s) => ({ ...s, error: null }));
    chrome.runtime.sendMessage({ action: 'reconnect' });
  }, []);

  return (
    <div className="app-container">
      <header className="app-header">
        <h1>go-mapi</h1>
        <div className="status">
          <span className={`status-dot ${state.connected ? 'connected' : 'disconnected'}`} />
          {state.connected ? 'Connected' : 'Disconnected'}
          {state.emails.length > 0 && ` • ${state.emails.length} pending`}
        </div>
      </header>

      {state.error && (
        <Alert
          variant="danger"
          className="error-alert"
          dismissible
          onClose={() => setState((s) => ({ ...s, error: null }))}
        >
          {state.error}
          {!state.connected && (
            <button
              className="btn btn-link btn-sm p-0 ms-2"
              onClick={handleReconnect}
            >
              Retry
            </button>
          )}
        </Alert>
      )}

      <div className="content">
        {state.loading ? (
          <div className="loading">
            <Spinner animation="border" variant="primary" />
          </div>
        ) : state.selectedEmail ? (
          <EmailDetail
            email={state.selectedEmail}
            onBack={handleBack}
            onCreateDraft={handleCreateDraft}
            onSend={handleSend}
            onDelete={handleDelete}
            sending={state.sending}
          />
        ) : (
          <EmailList emails={state.emails} onSelect={handleSelect} />
        )}
      </div>
    </div>
  );
}
