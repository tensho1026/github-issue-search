import type {
  AuthLogoutEnvelope,
  AuthSessionEnvelope,
} from "../../../shared/api/generated";
import { apiClient } from "../../../shared/api/client";
import { authEndpoints } from "../../../shared/config/app-config";

export function readAuthSession(signal?: AbortSignal) {
  return apiClient.get<AuthSessionEnvelope>(authEndpoints.session, { signal });
}

export function logoutAuthSession(csrfToken: string) {
  return apiClient.post<AuthLogoutEnvelope, Record<string, never>>(
    authEndpoints.logout,
    {},
    { headers: { "X-CSRF-Token": csrfToken } },
  );
}

export function refreshAuthSession(csrfToken: string) {
  return apiClient.post<AuthSessionEnvelope, Record<string, never>>(
    authEndpoints.refresh,
    {},
    { headers: { "X-CSRF-Token": csrfToken } },
  );
}
