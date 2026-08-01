import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";

import { Button } from "../../../components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "../../../components/ui/dialog";
import { Field } from "../../../components/ui/field";
import { Input } from "../../../components/ui/input";
import { ApiError } from "../../../shared/api/client";
import type {
  IssueSearchRequest,
  RepositoryDiscoveryRequest,
  SavedSearchWriteRequest,
} from "../../../shared/api/generated";
import { queryKeys } from "../../../shared/query/query-keys";
import { useAuth } from "../../auth/auth-context";
import { createSavedSearch } from "../api/account";
import { AccountRequestAlert } from "./AccountRequestAlert";

type Props =
  | { filters: IssueSearchRequest; searchType: "issue" }
  | { filters: RepositoryDiscoveryRequest; searchType: "repository" };

export function SaveSearchAction(props: Props) {
  const { markSessionExpired, session, signIn } = useAuth();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [formError, setFormError] = useState("");
  const mutation = useMutation({
    mutationFn: (request: SavedSearchWriteRequest) => {
      if (!session?.authenticated || !session.csrfToken) {
        throw new ApiError({
          code: "AUTHENTICATION_REQUIRED",
          message: "Authentication is required.",
          status: 401,
        });
      }
      return createSavedSearch(request, session.csrfToken);
    },
    async onError(error) {
      if (error instanceof ApiError && error.status === 401) {
        await markSessionExpired();
      }
    },
    async onSuccess() {
      setOpen(false);
      setName("");
      await queryClient.invalidateQueries({
        queryKey: queryKeys.account.savedSearches,
      });
    },
  });

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!name.trim()) {
      setFormError("Enter a name for this search.");
      return;
    }
    setFormError("");
    mutation.mutate({
      filters: props.filters,
      name: name.trim(),
      searchType: props.searchType,
    } as SavedSearchWriteRequest);
  }

  const authenticated = session?.authenticated && session.csrfToken;
  return (
    <>
      <Button onClick={() => setOpen(true)} size="small" variant="outline">
        {mutation.isSuccess ? "Search saved" : "Save this search"}
      </Button>
      <Dialog onOpenChange={setOpen} open={open}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {authenticated
                ? "Name this saved search"
                : "Save validated filters"}
            </DialogTitle>
            <DialogDescription>
              {authenticated
                ? "Only this validated filter definition is stored. Result payloads and anonymous history are excluded."
                : "Sign in to retain this filter definition. Your current URL and unsaved filter state will be preserved."}
            </DialogDescription>
          </DialogHeader>
          {authenticated ? (
            <form className="grid gap-4" onSubmit={submit}>
              <Field
                error={formError || undefined}
                htmlFor={`save-${props.searchType}-search-name`}
                label="Saved-search name"
              >
                <Input
                  id={`save-${props.searchType}-search-name`}
                  maxLength={80}
                  onChange={(event) => setName(event.target.value)}
                  value={name}
                />
              </Field>
              {mutation.error ? (
                <AccountRequestAlert error={mutation.error} />
              ) : null}
              <Button disabled={mutation.isPending} type="submit">
                {mutation.isPending ? "Saving…" : "Save search"}
              </Button>
            </form>
          ) : session?.configured === false ? (
            <p className="text-sm text-muted-foreground">
              Sign-in is not configured in this environment.
            </p>
          ) : (
            <Button onClick={() => signIn()}>Sign in with GitHub</Button>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
