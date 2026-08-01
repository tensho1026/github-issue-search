import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link } from "react-router";

import { Badge } from "../../../components/ui/badge";
import { Button } from "../../../components/ui/button";
import {
  Card,
  CardContent,
} from "../../../components/ui/card";
import { Field } from "../../../components/ui/field";
import { Input } from "../../../components/ui/input";
import { ApiError } from "../../../shared/api/client";
import type {
  IssueSearchRequest,
  SavedSearch,
  SavedSearchUpdateRequest,
} from "../../../shared/api/generated";
import { queryKeys } from "../../../shared/query/query-keys";
import {
  deleteSavedSearch,
  listSavedSearches,
  updateSavedSearch,
} from "../api/account";
import { savedSearchRoute } from "../model/saved-search-route";
import { AccountRequestAlert } from "./AccountRequestAlert";

type Props = {
  csrfToken: string;
  onSessionExpired: () => Promise<void>;
};

function updateRequest(
  search: SavedSearch,
  name: string,
): SavedSearchUpdateRequest {
  return search.searchType === "issue"
    ? {
        filters: search.filters as IssueSearchRequest,
        name,
        searchType: "issue",
        version: search.version,
      }
    : {
        filters: search.filters,
        name,
        searchType: "repository",
        version: search.version,
      };
}

function SavedSearchRow({
  disabled,
  onDelete,
  onRename,
  search,
}: {
  disabled: boolean;
  onDelete: (search: SavedSearch) => void;
  onRename: (search: SavedSearch, name: string) => void;
  search: SavedSearch;
}) {
  const [name, setName] = useState(search.name);
  return (
    <li>
      <Card>
        <CardContent className="grid gap-4 p-5 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
          <Field
            description={`Last updated ${new Date(
              search.updatedAt,
            ).toLocaleDateString()}.`}
            htmlFor={`saved-search-${search.id}`}
            label={
              <span className="inline-flex items-center gap-2">
                Search name
                <Badge variant="neutral">{search.searchType}</Badge>
              </span>
            }
          >
            <Input
              id={`saved-search-${search.id}`}
              maxLength={80}
              onChange={(event) => setName(event.target.value)}
              value={name}
            />
          </Field>
          <div className="flex flex-wrap gap-2">
            <Button asChild size="small" variant="outline">
              <Link to={savedSearchRoute(search)}>Run</Link>
            </Button>
            <Button
              disabled={disabled || !name.trim() || name === search.name}
              onClick={() => onRename(search, name.trim())}
              size="small"
              variant="secondary"
            >
              Rename
            </Button>
            <Button
              aria-label={`Delete saved search ${search.name}`}
              disabled={disabled}
              onClick={() => onDelete(search)}
              size="small"
              variant="danger"
            >
              Delete
            </Button>
          </div>
        </CardContent>
      </Card>
    </li>
  );
}

export function SavedSearchesPanel({ csrfToken, onSessionExpired }: Props) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryFn: ({ signal }) => listSavedSearches(signal),
    queryKey: queryKeys.account.savedSearches,
  });

  async function handleError(error: unknown) {
    if (error instanceof ApiError && error.status === 401) {
      await onSessionExpired();
    }
  }

  async function refresh() {
    await queryClient.invalidateQueries({
      queryKey: queryKeys.account.savedSearches,
    });
  }

  const update = useMutation({
    mutationFn: ({
      id,
      request,
    }: {
      id: string;
      request: SavedSearchUpdateRequest;
    }) => updateSavedSearch(id, request, csrfToken),
    onError: handleError,
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (search: SavedSearch) =>
      deleteSavedSearch(search.id, search.version, csrfToken),
    onError: handleError,
    onSuccess: refresh,
  });

  const mutationError = update.error ?? remove.error;
  const disabled = update.isPending || remove.isPending;

  return (
    <section aria-labelledby="saved-searches-heading" className="grid gap-5">
      <h2 className="sr-only" id="saved-searches-heading">
        Saved searches
      </h2>

      {mutationError ? <AccountRequestAlert error={mutationError} /> : null}
      {query.error ? <AccountRequestAlert error={query.error} /> : null}

      {query.isPending ? (
        <Card>
          <CardContent
            className="p-6 text-sm text-muted-foreground"
            role="status"
          >
            Loading saved searches…
          </CardContent>
        </Card>
      ) : query.data?.data.items.length === 0 ? (
        <Card>
          <CardContent className="grid justify-items-center gap-3 p-8 text-center">
            <p className="font-semibold">No saved searches yet</p>
            <p className="max-w-lg text-sm text-muted-foreground">
              Save the current validated filters from a public search page.
            </p>
          </CardContent>
        </Card>
      ) : (
        <ul className="grid gap-3">
          {query.data?.data.items.map((search) => (
            <SavedSearchRow
              disabled={disabled}
              key={`${search.id}-${search.version}`}
              onDelete={(item) => remove.mutate(item)}
              onRename={(item, nextName) =>
                update.mutate({
                  id: item.id,
                  request: updateRequest(item, nextName),
                })
              }
              search={search}
            />
          ))}
        </ul>
      )}
    </section>
  );
}
