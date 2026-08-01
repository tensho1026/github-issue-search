import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "../../../components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "../../../components/ui/dialog";
import { ApiError } from "../../../shared/api/client";
import type { BookmarkWriteRequest } from "../../../shared/api/generated";
import { queryKeys } from "../../../shared/query/query-keys";
import { useAuth } from "../../auth/auth-context";
import { upsertBookmark } from "../api/account";
import { AccountRequestAlert } from "./AccountRequestAlert";

export function BookmarkAction({ request }: { request: BookmarkWriteRequest }) {
  const { markSessionExpired, session, signIn } = useAuth();
  const queryClient = useQueryClient();
  const [promptOpen, setPromptOpen] = useState(false);
  const [savedTarget, setSavedTarget] = useState("");
  const targetKey = JSON.stringify(request);
  const mutation = useMutation({
    mutationFn: () => {
      if (!session?.authenticated || !session.csrfToken) {
        throw new ApiError({
          code: "AUTHENTICATION_REQUIRED",
          message: "Authentication is required.",
          status: 401,
        });
      }
      return upsertBookmark(request, session.csrfToken);
    },
    async onError(error) {
      if (error instanceof ApiError && error.status === 401) {
        await markSessionExpired();
        setPromptOpen(true);
      }
    },
    async onSuccess() {
      setSavedTarget(targetKey);
      await queryClient.invalidateQueries({
        queryKey: queryKeys.account.bookmarks,
      });
    },
  });

  const authenticated = session?.authenticated && session.csrfToken;
  return (
    <>
      <Button
        disabled={mutation.isPending}
        onClick={() => {
          if (authenticated) {
            mutation.mutate();
          } else {
            setPromptOpen(true);
          }
        }}
        variant="outline"
      >
        {mutation.isPending
          ? "Saving…"
          : savedTarget === targetKey
            ? "Bookmarked"
            : "Bookmark"}
      </Button>
      <Dialog onOpenChange={setPromptOpen} open={promptOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Save this reference to your workspace</DialogTitle>
            <DialogDescription>
              Bookmarks store only the repository and optional issue number.
              Sign-in is never required to keep exploring, and IssueScout will
              return to this exact page after authorization.
            </DialogDescription>
          </DialogHeader>
          {session?.configured === false ? (
            <p className="text-sm text-muted-foreground">
              Sign-in is not configured in this environment.
            </p>
          ) : (
            <Button
              onClick={() => {
                signIn();
              }}
            >
              Sign in with GitHub
            </Button>
          )}
        </DialogContent>
      </Dialog>
      {mutation.error &&
      (!(mutation.error instanceof ApiError) ||
        mutation.error.status !== 401) ? (
        <AccountRequestAlert error={mutation.error} />
      ) : null}
    </>
  );
}
