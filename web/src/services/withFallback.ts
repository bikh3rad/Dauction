import { config } from "./config";
import { isUnavailable, toApiError } from "./client";

// Tracks whether we are currently serving mock data, so the UI can surface an
// honest "offline / sample data" banner instead of pretending the backend is up.
let servingMock = config.forceMock;
const listeners = new Set<(mock: boolean) => void>();

export function isServingMock(): boolean {
  return servingMock;
}
export function onMockChange(fn: (mock: boolean) => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
function setServingMock(v: boolean) {
  if (servingMock === v) return;
  servingMock = v;
  listeners.forEach((l) => l(v));
}

/**
 * Run a real API call, falling back to a schema-accurate mock when:
 *   - VITE_USE_MOCK forces it, or
 *   - the backend is unavailable (network error / 5xx).
 *
 * Genuine business errors (4xx — e.g. "out of credits", "invalid code") are
 * RE-THROWN so the UI shows the real failure rather than masking it with mock
 * success. This keeps the app fully usable offline while staying honest online.
 */
export async function withFallback<T>(
  apiCall: () => Promise<T>,
  mock: () => T | Promise<T>,
): Promise<T> {
  if (config.forceMock) {
    setServingMock(true);
    return mock();
  }
  try {
    const result = await apiCall();
    setServingMock(false);
    return result;
  } catch (err) {
    if (isUnavailable(err)) {
      setServingMock(true);
      return mock();
    }
    // real business error: surface it
    throw toApiError(err);
  }
}
