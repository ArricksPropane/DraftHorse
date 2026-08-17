<!--
  DefaultsBanner — mailto-default nudge (ARRICKS-13).

  Design:
  - Rendered only when the backend reports the mailto UserChoice is NOT
    this app (the caller gates it, mirroring UpdateBanner's contract).
  - Windows deliberately blocks programmatic mailto-default changes
    (hash-protected UserChoice + the UCPD driver), so the ONLY honest
    affordance is a deep link into Settings > Default apps where one
    click finishes the job. The button does exactly that, nothing else.
  - Dismissible per session — the MAPI default (the scanner-critical
    one) is self-healed silently in Go; mailto only affects clicking
    mailto: links, so this must never nag.
-->
<script lang="ts">
  interface Props {
    onOpenSettings: () => void;
    onDismiss: () => void;
  }
  let { onOpenSettings, onDismiss }: Props = $props();
</script>

<section class="banner" aria-label="DraftHorse is not the default email link handler">
  <span class="msg">
    DraftHorse isn't your default app for email links yet
  </span>
  <span class="actions">
    <button type="button" class="open" onclick={onOpenSettings}>
      Open Windows Settings
    </button>
    <button type="button" class="dismiss" onclick={onDismiss} aria-label="Dismiss">
      ✕
    </button>
  </span>
</section>

<style>
  .banner {
    background: var(--c-surface-alt, #f4f4f6);
    color: var(--c-text, #222);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.4rem 1rem;
    gap: 1rem;
    font-size: 0.9rem;
    border-bottom: 1px solid var(--c-border, #ddd);
  }
  .msg {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .open {
    background: var(--c-accent);
    color: white;
    border: 0;
    padding: 0.3rem 0.75rem;
    border-radius: 4px;
    font-weight: 600;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .open:hover {
    filter: brightness(1.1);
  }
  .dismiss {
    background: transparent;
    color: inherit;
    border: 0;
    cursor: pointer;
    font-size: 0.85rem;
    padding: 0.2rem;
  }
</style>
