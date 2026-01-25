import '@testing-library/jest-dom';
import { vi, beforeEach, afterEach } from 'vitest';
import { chromeMock, resetChromeMocks } from './mocks/chrome';

// Stub global Chrome API
vi.stubGlobal('chrome', chromeMock);

// Mock fetch for Gmail API tests
const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

// Reset mocks between tests
beforeEach(() => {
  resetChromeMocks();
  mockFetch.mockReset();

  // Default successful fetch response
  mockFetch.mockResolvedValue({
    ok: true,
    json: () => Promise.resolve({ id: 'mock-id' }),
    text: () => Promise.resolve(''),
  });
});

afterEach(() => {
  vi.clearAllMocks();
});

// Export mockFetch for direct manipulation in tests
export { mockFetch };
