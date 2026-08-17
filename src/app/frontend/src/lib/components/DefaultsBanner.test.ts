// Component tests for DefaultsBanner.svelte — the ARRICKS-13 mailto-default
// nudge. The banner's whole contract: explain, offer the Settings deep link,
// stay dismissible. It must never claim it can set the default itself.
import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import DefaultsBanner from './DefaultsBanner.svelte';

describe('DefaultsBanner', () => {
  it('renders the nudge copy and the Settings action', () => {
    const { getByText, getByRole } = render(DefaultsBanner, {
      props: { onOpenSettings: vi.fn(), onDismiss: vi.fn() },
    });
    expect(getByText(/isn't your default app for email links/i)).toBeInTheDocument();
    expect(getByRole('button', { name: /open windows settings/i })).toBeInTheDocument();
  });

  it('calls onOpenSettings when the Settings button is clicked', async () => {
    const onOpenSettings = vi.fn();
    const { getByRole } = render(DefaultsBanner, {
      props: { onOpenSettings, onDismiss: vi.fn() },
    });
    await fireEvent.click(getByRole('button', { name: /open windows settings/i }));
    expect(onOpenSettings).toHaveBeenCalledOnce();
  });

  it('calls onDismiss when dismissed', async () => {
    const onDismiss = vi.fn();
    const { getByRole } = render(DefaultsBanner, {
      props: { onOpenSettings: vi.fn(), onDismiss },
    });
    await fireEvent.click(getByRole('button', { name: /dismiss/i }));
    expect(onDismiss).toHaveBeenCalledOnce();
  });
});
