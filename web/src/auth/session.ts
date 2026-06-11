// Client-side session store. In the deployed demo the gateway/identity stack
// isn't running, so the session is persisted in localStorage and the mock
// identity handlers read/write it. The bearer token is the account id (the
// gateway's dev auth scheme); production swaps this for a real JWT.

import type { Account } from "@/types";

const KEY = "dauction.session";

export interface Session {
  token: string;
  account: Account;
}

// A logged-out visitor: browses the gallery as GUEST, cannot participate.
export function guestAccount(): Account {
  return {
    id: "guest",
    tier: "GUEST",
    kycStatus: "PENDING",
    eligible: false,
    roles: [],
    status: "REGISTERED",
    mobileE164: "",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };
}

export function getSession(): Session | null {
  try {
    const raw = localStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}

/** hasSession reports whether the user has interacted with auth at all (logged
 *  in OR explicitly chose to browse as guest). Drives the first-run redirect. */
export function hasSession(): boolean {
  return getSession() !== null;
}

export function currentAccount(): Account {
  return getSession()?.account ?? guestAccount();
}

export function setSession(account: Account): Session {
  const s: Session = { token: account.id, account };
  localStorage.setItem(KEY, JSON.stringify(s));
  return s;
}

/** Persist the "browse as guest" choice so the login page isn't shown again. */
export function continueAsGuest(): Session {
  return setSession(guestAccount());
}

export function signOut(): void {
  localStorage.removeItem(KEY);
}

export function isAuthed(): boolean {
  const s = getSession();
  return !!s && s.account.id !== "guest";
}
