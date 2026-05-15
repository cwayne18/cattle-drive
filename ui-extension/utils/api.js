export const DEFAULT_PROXY_API_BASE = '/k8s/clusters/local/api/v1/namespaces/cattle-system/services/http:cattle-drive-api:8080/proxy';

/**
 * Build request headers for a cattle-drive API call.
 *
 * For same-origin proxy paths (apiBase starts with '/') the browser session
 * cookie is used automatically via `credentials: 'same-origin'`, so no
 * Authorization header is needed or desirable.
 *
 * For external API bases (e.g. http://localhost:8080 during development) we
 * attach a Bearer token from the Rancher auth store when one is available.
 *
 * @param {object} store - Vuex store instance
 * @param {string} apiBase - The API base URL in use for this request
 * @returns {Record<string, string>}
 */
export function authHeaders(store, apiBase) {
  const headers = { 'Content-Type': 'application/json' };

  // Same-origin proxy path — rely on cookie session, skip manual auth header.
  if (!apiBase || apiBase.startsWith('/')) {
    return headers;
  }

  // External API base — attach Bearer token when available.
  const rawToken = store?.getters?.['auth/token'];
  const token = typeof rawToken === 'string' ? rawToken : (rawToken?.token || rawToken?.value);

  if (token) {
    headers.Authorization = `Bearer ${ token }`;
  }

  return headers;
}
