import { vi } from 'vitest';

// Mock Chrome runtime.Port
export const createMockPort = () => ({
  name: 'com.gomapi.host',
  onMessage: {
    addListener: vi.fn(),
    removeListener: vi.fn(),
  },
  onDisconnect: {
    addListener: vi.fn(),
    removeListener: vi.fn(),
  },
  postMessage: vi.fn(),
  disconnect: vi.fn(),
});

// Mock Chrome APIs
export const chromeMock = {
  runtime: {
    connectNative: vi.fn(() => createMockPort()),
    sendMessage: vi.fn(() => Promise.resolve()),
    onMessage: {
      addListener: vi.fn(),
      removeListener: vi.fn(),
      hasListener: vi.fn(() => false),
    },
    lastError: null as chrome.runtime.LastError | null,
    id: 'mock-extension-id',
  },
  identity: {
    getAuthToken: vi.fn((options: { interactive: boolean }, callback: (token?: string) => void) => {
      callback('mock-auth-token');
    }),
    removeCachedAuthToken: vi.fn((details: { token: string }, callback?: () => void) => {
      callback?.();
    }),
  },
  action: {
    setBadgeText: vi.fn(),
    setBadgeBackgroundColor: vi.fn(),
  },
  tabs: {
    create: vi.fn(() => Promise.resolve({ id: 1 })),
  },
  storage: {
    local: {
      get: vi.fn(() => Promise.resolve({})),
      set: vi.fn(() => Promise.resolve()),
    },
    sync: {
      get: vi.fn(() => Promise.resolve({})),
      set: vi.fn(() => Promise.resolve()),
    },
  },
};

// Helper to reset all mocks
export const resetChromeMocks = () => {
  chromeMock.runtime.connectNative.mockClear();
  chromeMock.runtime.sendMessage.mockClear();
  chromeMock.runtime.onMessage.addListener.mockClear();
  chromeMock.runtime.lastError = null;
  chromeMock.identity.getAuthToken.mockClear();
  chromeMock.action.setBadgeText.mockClear();
  chromeMock.action.setBadgeBackgroundColor.mockClear();
  chromeMock.tabs.create.mockClear();
};

// Helper to simulate auth token error
export const mockAuthTokenError = (errorMessage: string) => {
  chromeMock.runtime.lastError = { message: errorMessage };
  chromeMock.identity.getAuthToken.mockImplementation(
    (_options: { interactive: boolean }, callback: (token?: string) => void) => {
      callback(undefined);
    }
  );
};

// Helper to simulate successful auth
export const mockAuthTokenSuccess = (token: string = 'mock-auth-token') => {
  chromeMock.runtime.lastError = null;
  chromeMock.identity.getAuthToken.mockImplementation(
    (_options: { interactive: boolean }, callback: (token?: string) => void) => {
      callback(token);
    }
  );
};
