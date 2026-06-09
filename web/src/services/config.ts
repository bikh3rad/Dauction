// Environment-driven runtime configuration.
// All values come from Vite env (import.meta.env), with sane production defaults.

function bool(v: string | undefined, dflt = false): boolean {
  if (v == null) return dflt;
  return v === "1" || v.toLowerCase() === "true";
}

export const config = {
  /** Base path/URL for API calls. Dev: same-origin "/apis" (Vite-proxied). */
  apiBase: import.meta.env.VITE_API_BASE ?? "/apis",
  /** Force the mock layer instead of the real backend. */
  forceMock: bool(import.meta.env.VITE_USE_MOCK, false),
  /** Dev auth: the gateway treats the Bearer token AS the account UUID. */
  devAccountId:
    import.meta.env.VITE_DEV_ACCOUNT_ID ??
    "0a7a4e00-0000-4000-8000-00000000a74e",
  /** Request timeout (ms) before we fall back to mock. */
  timeoutMs: 6000,
};

export type AppConfig = typeof config;
