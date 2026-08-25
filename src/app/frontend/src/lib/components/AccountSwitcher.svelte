<script lang="ts">
  /* V4 two-account switcher (docs/V4-PLAN.md Phase 2).
   *
   * Radio semantics: exactly one account is ACTIVE and every scan drafts to
   * it. Signed-out slots offer "Sign in" (the add-second-account flow);
   * switching never prompts during a scan — this switch and the tray rows
   * are the only two places the choice is made. */
  import type { AccountInfo } from '../auth';

  let { accounts, onActivate, onAddAccount, onSignOutSlot }: {
    accounts: AccountInfo[];
    onActivate: (slot: number) => void;
    onAddAccount: (slot: number) => void;
    onSignOutSlot: (slot: number) => void;
  } = $props();
</script>

<div class="switcher" role="radiogroup" aria-label="Account scans draft to">
  {#each accounts as acct (acct.slot)}
    <div class="row" class:active={acct.active}>
      {#if acct.authenticated}
        <label class="pick">
          <input
            type="radio"
            name="active-account"
            checked={acct.active}
            onchange={() => onActivate(acct.slot)}
          />
          <span class="email">{acct.email || acct.name || `Account ${acct.slot + 1}`}</span>
          {#if acct.active}<span class="badge">drafting here</span>{/if}
        </label>
        <button type="button" class="link" onclick={() => onSignOutSlot(acct.slot)}>
          Sign out
        </button>
      {:else}
        <span class="email muted">Account {acct.slot + 1} — not signed in</span>
        <button type="button" class="link" onclick={() => onAddAccount(acct.slot)}>
          Sign in
        </button>
      {/if}
    </div>
  {/each}
</div>

<style>
  .switcher {
    display: flex;
    flex-direction: column;
    gap: var(--space-xs, 4px);
    padding: var(--space-sm) var(--space-md);
    border-bottom: 1px solid var(--c-border);
  }

  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-md);
    font-size: 14px;
  }

  .pick {
    display: flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;
  }

  .email { color: var(--c-text); }
  .muted { color: var(--c-text-muted); }

  .badge {
    font-size: 12px;
    color: var(--c-accent);
    border: 1px solid var(--c-accent);
    border-radius: 999px;
    padding: 0 8px;
  }

  .link {
    background: transparent;
    border: none;
    color: var(--c-text-muted);
    cursor: pointer;
    font-size: 13px;
    font-family: inherit;
    text-decoration: underline;
  }

  .link:hover { color: var(--c-text); }

  .link:focus-visible {
    outline: 2px solid var(--c-accent);
    outline-offset: 2px;
  }
</style>
