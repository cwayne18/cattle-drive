// Persist cattle-drive navigation state in sessionStorage so that sensitive
// values (apiBase, source, target, kubeconfig) are never placed in the URL.
// sessionStorage is scoped to the browser tab and cleared when the tab closes.

const KEY = 'cattle-drive-nav';

/**
 * Save the current navigation state.
 * @param {{ source: string, target: string, apiBase: string, kubeconfig: string }} data
 */
export function saveNavState(data) {
  try {
    sessionStorage.setItem(KEY, JSON.stringify(data));
  } catch (_) {
    // sessionStorage unavailable (e.g. private-browsing quota exceeded) — silently ignore.
  }
}

/**
 * Load previously saved navigation state, or null if none exists.
 * @returns {{ source: string, target: string, apiBase: string, kubeconfig: string } | null}
 */
export function loadNavState() {
  try {
    const raw = sessionStorage.getItem(KEY);

    return raw ? JSON.parse(raw) : null;
  } catch (_) {
    return null;
  }
}

/**
 * Remove the saved navigation state.
 */
export function clearNavState() {
  try {
    sessionStorage.removeItem(KEY);
  } catch (_) {}
}
