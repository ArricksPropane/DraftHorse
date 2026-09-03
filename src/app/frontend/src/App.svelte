<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { EventsOn } from '../wailsjs/runtime/runtime';
  import { CreateDraftForID, DismissEmail } from '../wailsjs/go/main/App';
  import { subscribeQueue, fetchQueue, type EmailWithId } from './lib/queue';
  import {
    fetchAuthStatus,
    subscribeAuth,
    signIn,
    signOut,
    hasSeenPreAuthExplainer,
    markPreAuthExplainerSeen,
    fetchAccounts,
    setActiveAccount,
    signInAccount,
    signOutAccount,
    type AuthStatus,
    type AccountInfo,
  } from './lib/auth';
  import {
    fetchSettings,
    setMode as persistMode,
    getPausedState,
    subscribeAutoDraftResult,
    subscribePauseChanged,
    fetchUpdateState,
    subscribeUpdateState,
    fetchDefaultsStatus,
    openDefaultAppsSettings,
    type Mode,
    type ErrorCategory,
    type AutoDraftResult,
    type UpdateState,
    type DefaultsStatus,
  } from './lib/settings';
  import SignInScreen from './lib/components/SignInScreen.svelte';
  import PreAuthModal from './lib/components/PreAuthModal.svelte';
  import ReAuthBanner from './lib/components/ReAuthBanner.svelte';
  import SignedInHeader from './lib/components/SignedInHeader.svelte';
  import AccountSwitcher from './lib/components/AccountSwitcher.svelte';
  import QueueRow from './lib/components/QueueRow.svelte';
  import UpdateBanner from './lib/components/UpdateBanner.svelte';
  import UpdatePanel from './lib/components/UpdatePanel.svelte';
  import DefaultsBanner from './lib/components/DefaultsBanner.svelte';
  import './lib/styles.css';

  // Existing state
  let queue = $state<EmailWithId[]>([]);
  let errorMsg = $state<string | null>(null);
  let auth = $state<AuthStatus>({ authenticated: false });
  // V4 two-account roster. Refreshed on every auth-changed event — slot
  // sign-in/out and active switches all funnel through that one event.
  let accounts = $state<AccountInfo[]>([]);

  async function refreshAccounts() {
    try {
      accounts = await fetchAccounts();
    } catch {
      // Non-fatal — the switcher just doesn't render until the next event.
    }
  }

  // Each binding emits auth-changed, whose handler refreshes the roster —
  // no second round-trip here.
  async function handleActivateAccount(slot: number) {
    await setActiveAccount(slot);
  }

  async function handleAddAccount(slot: number) {
    try {
      await signInAccount(slot);
    } catch {
      // Cancelled/failed flow: auth-changed did not fire; re-sync the roster.
      await refreshAccounts();
    }
  }

  async function handleSignOutSlot(slot: number) {
    await signOutAccount(slot);
  }
  let showPreAuthModal = $state(false);
  let showReAuthBanner = $state(false);
  let wasAuthenticated = false;

  // Phase 9 state
  let mode = $state<Mode>('manual');
  let paused = $state(false);
  let autoDraftErrors = $state(new Map<string, ErrorCategory>());
  // QUICK-260423-tk6: parallel map of raw Go error text per failed emailId.
  // Populated on auto-draft-result failure, cleared on success or queue prune.
  // Passed through to QueueRow → AutoDraftErrorBadge so the badge tooltip /
  // subtitle can show *why* the draft failed (not just the opaque category).
  let autoDraftReasons = $state(new Map<string, string>());
  let flashingIds = $state(new Set<string>());
  let inflightIds = $state(new Set<string>());

  // ARRICKS-13 state — mailto-default nudge. The MAPI default self-heals
  // silently in Go; this banner covers only the Settings-gated mailto half.
  let defaultsStatus = $state<DefaultsStatus | null>(null);
  let defaultsBannerDismissed = $state(false);

  // Phase 11-03 state — notify-only update UX (D-01/D-02/D-07/D-08).
  let updateState = $state<UpdateState | null>(null);
  let showUpdatePanel = $state(false);
  let bannerDismissedForVersion = $state<string | null>(null);

  // Collect all unsub functions for cleanup
  const unsubs: Array<() => void> = [];

  onMount(async () => {
    // Fetch initial state in parallel.
    const [initialAuth, initialQueue, initialSettings, initialPaused, initialUpdate, initialDefaults] =
      await Promise.all([
        fetchAuthStatus(),
        fetchQueue().catch((e) => { errorMsg = (e as Error).message; return []; }),
        fetchSettings().catch(() => ({ mode: 'manual' as Mode })),
        getPausedState().catch(() => false),
        // D-04 silent-failure: never block startup on update hydration.
        fetchUpdateState().catch(() => null),
        // ARRICKS-13: same silent-failure rule — a failed defaults read just
        // hides the banner.
        fetchDefaultsStatus().catch(() => null),
      ]);

    auth = initialAuth as AuthStatus;
    wasAuthenticated = auth.authenticated;
    // V4: hydrate the account roster after the critical state (non-blocking).
    void refreshAccounts();
    queue = initialQueue as EmailWithId[];
    mode = ((initialSettings as { mode: string }).mode === 'auto-draft' ? 'auto-draft' : 'manual');
    paused = initialPaused as boolean;
    updateState = initialUpdate as UpdateState | null;
    defaultsStatus = initialDefaults as DefaultsStatus | null;

    // Subscribe to queue updates — prune stale state entries on each update.
    unsubs.push(subscribeQueue(
      (next) => {
        queue = next;
        // Reassign Maps/Sets to guarantee Svelte 5 detects the change when pruning.
        const ids = new Set(next.map((e) => e.id));
        const nextErrors = new Map(autoDraftErrors);
        let errChanged = false;
        for (const id of nextErrors.keys()) {
          if (!ids.has(id)) { nextErrors.delete(id); errChanged = true; }
        }
        if (errChanged) autoDraftErrors = nextErrors;

        // Prune the tk6 reasons map in lockstep with autoDraftErrors.
        const nextReasons = new Map(autoDraftReasons);
        let reasonChanged = false;
        for (const id of nextReasons.keys()) {
          if (!ids.has(id)) { nextReasons.delete(id); reasonChanged = true; }
        }
        if (reasonChanged) autoDraftReasons = nextReasons;

        const nextFlashing = new Set(flashingIds);
        let flashChanged = false;
        for (const id of nextFlashing) {
          if (!ids.has(id)) { nextFlashing.delete(id); flashChanged = true; }
        }
        if (flashChanged) flashingIds = nextFlashing;

        const nextInflight = new Set(inflightIds);
        let inflightChanged = false;
        for (const id of nextInflight) {
          if (!ids.has(id)) { nextInflight.delete(id); inflightChanged = true; }
        }
        if (inflightChanged) inflightIds = nextInflight;
      },
      (e) => { errorMsg = (e as Error)?.message ?? 'queue fetch failed'; },
    ));

    // Queue error events from Go
    unsubs.push(EventsOn('queue-error', (msg: string) => { errorMsg = msg; }));

    // Auth state changes — trigger re-auth banner on sign-out.
    unsubs.push(subscribeAuth(async (s) => {
      const becameSignedOut = wasAuthenticated && !s.authenticated;
      auth = s;
      // V4: every slot sign-in/out and active switch emits auth-changed —
      // one hook keeps the switcher current. Refresh BEFORE deciding on the
      // banner: switching to an empty slot while another slot is signed in
      // is an account switch, not an expired session (review 2026-08-28).
      await refreshAccounts();
      const otherSlotSignedIn = accounts.some((a) => a.authenticated);
      if (becameSignedOut && !otherSlotSignedIn) {
        showReAuthBanner = true;
      } else if (s.authenticated || otherSlotSignedIn) {
        showReAuthBanner = false;
      }
      wasAuthenticated = s.authenticated;
    }));

    // Auto-draft result (fires for both manual CreateDraftForID and automode).
    unsubs.push(subscribeAutoDraftResult((r: AutoDraftResult) => {
      // Reassign to guarantee Svelte 5 fine-grained reactivity detects the change.
      inflightIds = new Set([...inflightIds].filter((id) => id !== r.emailId));
      if (r.success) {
        const next = new Map(autoDraftErrors);
        next.delete(r.emailId);
        autoDraftErrors = next;
        // Clear tk6 reason tracking on success too — the row will flash green
        // (if visible) then leave the queue on the next queue-update.
        const nextReasons = new Map(autoDraftReasons);
        if (nextReasons.delete(r.emailId)) autoDraftReasons = nextReasons;
        // D-04: only flash in-window when visible + focused; Go fires toast when hidden.
        if (isWindowVisibleAndFocused()) {
          flashingIds = new Set([...flashingIds, r.emailId]);
          setTimeout(() => {
            flashingIds = new Set([...flashingIds].filter((id) => id !== r.emailId));
          }, 1600);
        }
      } else if (r.errorCategory) {
        const next = new Map(autoDraftErrors);
        next.set(r.emailId, r.errorCategory);
        autoDraftErrors = next;
        // QUICK-260423-tk6: stash raw reason so AutoDraftErrorBadge can show
        // it. Gracefully tolerate missing reason (older Go builds).
        if (r.reason) {
          const nextReasons = new Map(autoDraftReasons);
          nextReasons.set(r.emailId, r.reason);
          autoDraftReasons = nextReasons;
        }
      }
    }));

    // Pause state changes from Go (tray menu or PauseWatching/ResumeWatching calls).
    unsubs.push(subscribePauseChanged((p: boolean) => { paused = p; }));

    // Update state changes from Go (startup check, 24h scheduler, manual check).
    // Plan 11-01 guarantees one event per material state change.
    unsubs.push(subscribeUpdateState((s: UpdateState) => { updateState = s; }));
  });

  onDestroy(() => {
    for (const u of unsubs) u();
  });

  /** D-04: visible + focused proxy using web platform APIs supported by WebView2. */
  function isWindowVisibleAndFocused(): boolean {
    return document.visibilityState === 'visible' && document.hasFocus();
  }

  /** Compute per-row UI state from the three tracking sets/maps. */
  function rowStateFor(id: string): 'idle' | 'in-flight' | 'drafted-flash' | 'error' {
    if (flashingIds.has(id)) return 'drafted-flash';
    if (inflightIds.has(id)) return 'in-flight';
    if (autoDraftErrors.has(id)) return 'error';
    return 'idle';
  }

  /** Manual draft: put row in-flight, call binding; auto-draft-result event resolves state. */
  async function handleCreateDraft(id: string) {
    inflightIds = new Set([...inflightIds, id]);
    try {
      await CreateDraftForID(id);
      // auto-draft-result event will handle success/failure state update.
    } catch {
      // Binding threw (network/IPC error) — clear in-flight, show generic gmail error.
      inflightIds = new Set([...inflightIds].filter((x) => x !== id));
      const next = new Map(autoDraftErrors);
      next.set(id, 'gmail');
      autoDraftErrors = next;
    }
  }

  /** Dismiss: call binding; queue-update handles row removal. */
  async function handleDismiss(id: string) {
    try { await DismissEmail(id); } catch { /* ignore dismiss errors */ }
  }

  /** Mode toggle: persist then update local state. */
  async function handleModeChange(next: Mode) {
    await persistMode(next);
    mode = next;
  }

  // Auth flow handlers — unchanged from Phase 8.
  async function handleSignInClick() {
    if (!hasSeenPreAuthExplainer()) {
      showPreAuthModal = true;
      return;
    }
    await signIn();
  }

  async function handlePreAuthContinue() {
    markPreAuthExplainerSeen();
    showPreAuthModal = false;
    await signIn();
  }

  function handlePreAuthCancel() {
    showPreAuthModal = false;
  }

  async function handleReAuthClick() {
    showReAuthBanner = false;
    await signIn();
  }

  async function handleSignOutClick() {
    await signOut();
  }

  /** Phase 11-03: open the update panel from the banner (D-01 → D-02). */
  function handleOpenUpdatePanel() {
    showUpdatePanel = true;
  }

  /** Phase 11-03: close the panel without dismissing the banner — the
   *  banner stays until a newer version actually ships. bannerDismissedForVersion
   *  is reserved for an optional future "hide until next release" dismissal
   *  flow; we keep it inert this phase to avoid suppressing legitimate
   *  upgrade prompts. */
  function handleCloseUpdatePanel() {
    showUpdatePanel = false;
    // bannerDismissedForVersion intentionally not updated here (D-01: banner
    // remains persistent across panel open/close cycles).
    void bannerDismissedForVersion; // silence "assigned but never read" without removing the state field
  }

  /** ARRICKS-13: deep-link to Settings > Default apps, then re-check on a
   *  short delay so the banner clears by itself once the user picks
   *  DraftHorse there. Fire-and-forget; failures just leave the banner. */
  function handleOpenDefaultApps() {
    void openDefaultAppsSettings().catch(() => undefined);
    setTimeout(() => {
      void fetchDefaultsStatus()
        .then((s) => { defaultsStatus = s; })
        .catch(() => undefined);
    }, 15000);
  }
</script>

{#if updateState && updateState.updateAvailable}
  <UpdateBanner
    latestVersion={updateState.latestVersion}
    onViewUpdate={handleOpenUpdatePanel}
  />
{/if}

{#if auth.authenticated && defaultsStatus && !defaultsStatus.mailtoDefault && !defaultsBannerDismissed}
  <DefaultsBanner
    onOpenSettings={handleOpenDefaultApps}
    onDismiss={() => { defaultsBannerDismissed = true; }}
  />
{/if}

{#if showReAuthBanner}
  <ReAuthBanner onRestore={handleReAuthClick} />
{/if}

{#if auth.authenticated}
  <SignedInHeader
    email={auth.email ?? ''}
    name={auth.name ?? ''}
    onSignOut={handleSignOutClick}
    {mode}
    onModeChange={handleModeChange}
  />
{/if}
{#if accounts.some((a) => a.authenticated)}
  <!-- Shown whenever ANY slot is signed in — including while the ACTIVE
       slot is empty (adding an account), so the window can always switch
       back without the tray. -->
  <AccountSwitcher
    {accounts}
    onActivate={handleActivateAccount}
    onAddAccount={handleAddAccount}
    onSignOutSlot={handleSignOutSlot}
  />
{/if}

<main>
  {#if !auth.authenticated}
    <SignInScreen onSignIn={handleSignInClick} />
  {:else if errorMsg}
    <section class="state state--error">
      <h2>Watcher stopped</h2>
      <p>DraftHorse can't watch %LOCALAPPDATA%\DraftHorse\queue\. Restart the app, or check app.log for details.</p>
    </section>
  {:else if queue.length === 0}
    <section class="state state--empty">
      <h2>No emails waiting</h2>
      <p>When a Windows app sends to mail, it will appear here.</p>
    </section>
  {:else}
    <ul class="queue" aria-live="polite">
      {#each queue as item (item.id)}
        <QueueRow
          {item}
          state={rowStateFor(item.id)}
          authenticated={auth.authenticated}
          errorCategory={autoDraftErrors.get(item.id)}
          errorReason={autoDraftReasons.get(item.id)}
          onCreateDraft={handleCreateDraft}
          onDismiss={handleDismiss}
        />
      {/each}
    </ul>
  {/if}
</main>

{#if showPreAuthModal}
  <PreAuthModal onContinue={handlePreAuthContinue} onCancel={handlePreAuthCancel} />
{/if}

{#if showUpdatePanel && updateState}
  <UpdatePanel update={updateState} onClose={handleCloseUpdatePanel} />
{/if}
