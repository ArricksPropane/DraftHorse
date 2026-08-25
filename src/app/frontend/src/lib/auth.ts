import { EventsOn } from '../../wailsjs/runtime/runtime';
import {
  GetAuthStatus,
  SignIn,
  SignOut,
  GetAccounts,
  SetActiveAccount,
  SignInAccount,
  SignOutAccount,
} from '../../wailsjs/go/main/App';
import type { main } from '../../wailsjs/go/models';

export type AuthStatus = main.AuthStatus;
export type AccountInfo = main.AccountInfo;

/** V4 two-account support: the roster behind the account switcher. */
export async function fetchAccounts(): Promise<AccountInfo[]> {
  return await GetAccounts();
}

/** Make `slot` the account scans draft to. Allowed for signed-out slots —
 * the app then shows the sign-in screen for it (the add-account flow). */
export async function setActiveAccount(slot: number): Promise<void> {
  await SetActiveAccount(slot);
}

/** Run the OAuth flow for one slot without changing which is active. */
export async function signInAccount(slot: number): Promise<void> {
  await SignInAccount(slot);
}

/** Sign one slot out; the other account is untouched. */
export async function signOutAccount(slot: number): Promise<void> {
  await SignOutAccount(slot);
}

const PREAUTH_SEEN_KEY = 'DraftHorse.preauth-seen';

export function hasSeenPreAuthExplainer(): boolean {
  try {
    return localStorage.getItem(PREAUTH_SEEN_KEY) === '1';
  } catch {
    return false;
  }
}

export function markPreAuthExplainerSeen(): void {
  try {
    localStorage.setItem(PREAUTH_SEEN_KEY, '1');
  } catch {
    // ignore — non-fatal
  }
}

/**
 * Fetch the current auth status from Go. Safe to call at any time.
 */
export async function fetchAuthStatus(): Promise<AuthStatus> {
  return await GetAuthStatus();
}

/**
 * Subscribe to auth-changed events emitted by Go (startup, sign-in, sign-out,
 * invalid_grant). Returns an unsubscribe function.
 */
export function subscribeAuth(cb: (status: AuthStatus) => void): () => void {
  return EventsOn('auth-changed', (s: AuthStatus) => cb(s));
}

export async function signIn(): Promise<void> {
  await SignIn();
}

export async function signOut(): Promise<void> {
  await SignOut();
}
