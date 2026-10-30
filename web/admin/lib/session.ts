// Browser session state.
//
// The real session token now lives in an HttpOnly cookie the server sets on
// login (and on Telegram OIDC callback); JavaScript never sees it, so it can't
// be exfiltrated by an XSS. This module keeps only a NON-secret boolean flag so
// client-side guards can decide what to render without a round-trip. `/auth/me`
// remains the source of truth — the flag just avoids a login-screen flash.

const AUTHED_KEY = "axo_authed";

export function markAuthed(): void {
  try {
    localStorage.setItem(AUTHED_KEY, "1");
  } catch {
    /* storage disabled — guards fall back to /auth/me */
  }
}

export function clearAuthed(): void {
  try {
    localStorage.removeItem(AUTHED_KEY);
  } catch {
    /* ignore */
  }
  // Migrate away any legacy token that older builds stored.
  try {
    localStorage.removeItem("token");
  } catch {
    /* ignore */
  }
  // Drop the server's readable flag too, so a 401 doesn't leave the guards
  // believing there is still a session (which would bounce-loop).
  if (typeof document !== "undefined") {
    document.cookie = `${AUTHED_KEY}=; Path=/; Max-Age=0; SameSite=Lax`;
  }
}

// hasFlagCookie reads the non-secret companion cookie the server sets next to
// the HttpOnly session cookie. This is the authoritative signal: a session can
// be established by a server-side redirect (the Telegram OIDC callback) where
// no JavaScript runs to record it in localStorage.
function hasFlagCookie(): boolean {
  if (typeof document === "undefined") return false;
  return document.cookie
    .split(";")
    .some((c) => c.trim() === `${AUTHED_KEY}=1`);
}

export function isAuthed(): boolean {
  if (hasFlagCookie()) return true;
  try {
    // Treat a legacy token as authed too, so a session mid-upgrade isn't logged
    // out; it is cleared on the next 401 or logout.
    return (
      localStorage.getItem(AUTHED_KEY) === "1" ||
      !!localStorage.getItem("token")
    );
  } catch {
    return false;
  }
}
