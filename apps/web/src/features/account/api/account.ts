import type {
  AccountDeleteEnvelope,
  AccountDeleteRequest,
  AccountExportEnvelope,
  BookmarkEnvelope,
  BookmarkListEnvelope,
  BookmarkWriteRequest,
  DeletionEnvelope,
  PreferencesEnvelope,
  PreferencesWriteRequest,
  SavedSearchEnvelope,
  SavedSearchListEnvelope,
  SavedSearchUpdateRequest,
  SavedSearchWriteRequest,
} from "../../../shared/api/generated";
import { apiClient } from "../../../shared/api/client";
import { accountEndpoints } from "../../../shared/config/app-config";

function csrfOptions(csrfToken: string) {
  return { headers: { "X-CSRF-Token": csrfToken } };
}

export function listBookmarks(signal?: AbortSignal) {
  return apiClient.get<BookmarkListEnvelope>(accountEndpoints.bookmarks(), {
    signal,
  });
}

export function upsertBookmark(
  request: BookmarkWriteRequest,
  csrfToken: string,
) {
  return apiClient.put<BookmarkEnvelope, BookmarkWriteRequest>(
    accountEndpoints.bookmarks(),
    request,
    csrfOptions(csrfToken),
  );
}

export function deleteBookmark(id: string, version: number, csrfToken: string) {
  return apiClient.delete<DeletionEnvelope>(
    accountEndpoints.bookmark(id, version),
    undefined,
    csrfOptions(csrfToken),
  );
}

export function listSavedSearches(signal?: AbortSignal) {
  return apiClient.get<SavedSearchListEnvelope>(
    accountEndpoints.savedSearches(),
    { signal },
  );
}

export function createSavedSearch(
  request: SavedSearchWriteRequest,
  csrfToken: string,
) {
  return apiClient.post<SavedSearchEnvelope, SavedSearchWriteRequest>(
    accountEndpoints.savedSearches(),
    request,
    csrfOptions(csrfToken),
  );
}

export function updateSavedSearch(
  id: string,
  request: SavedSearchUpdateRequest,
  csrfToken: string,
) {
  return apiClient.put<SavedSearchEnvelope, SavedSearchUpdateRequest>(
    accountEndpoints.savedSearch(id),
    request,
    csrfOptions(csrfToken),
  );
}

export function deleteSavedSearch(
  id: string,
  version: number,
  csrfToken: string,
) {
  return apiClient.delete<DeletionEnvelope>(
    accountEndpoints.savedSearchForDelete(id, version),
    undefined,
    csrfOptions(csrfToken),
  );
}

export function getPreferences(signal?: AbortSignal) {
  return apiClient.get<PreferencesEnvelope>(accountEndpoints.preferences, {
    signal,
  });
}

export function updatePreferences(
  request: PreferencesWriteRequest,
  csrfToken: string,
) {
  return apiClient.put<PreferencesEnvelope, PreferencesWriteRequest>(
    accountEndpoints.preferences,
    request,
    csrfOptions(csrfToken),
  );
}

export function exportAccount(signal?: AbortSignal) {
  return apiClient.get<AccountExportEnvelope>(accountEndpoints.export, {
    signal,
  });
}

export function deleteAccount(csrfToken: string) {
  const request: AccountDeleteRequest = { confirmation: "DELETE" };
  return apiClient.delete<AccountDeleteEnvelope, AccountDeleteRequest>(
    accountEndpoints.account,
    request,
    csrfOptions(csrfToken),
  );
}
