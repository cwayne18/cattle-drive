export const DEFAULT_PROXY_API_BASE = '/k8s/clusters/local/api/v1/namespaces/cattle-system/services/http:cattle-drive-api:8080/proxy';

export function authHeaders(store) {
  const headers = { 'Content-Type': 'application/json' };
  const rawToken = store?.getters?.['auth/token'];
  const token = typeof rawToken === 'string' ? rawToken : (rawToken?.token || rawToken?.value);
  if (token) {
    headers.Authorization = `Bearer ${ token }`;
  }
  return headers;
}
