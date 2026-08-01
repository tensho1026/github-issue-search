import { useMutation } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "../../../components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "../../../components/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "../../../components/ui/dialog";
import { Field } from "../../../components/ui/field";
import { Input } from "../../../components/ui/input";
import { ApiError } from "../../../shared/api/client";
import { deleteAccount, exportAccount } from "../api/account";
import { AccountRequestAlert } from "./AccountRequestAlert";

type Props = {
  csrfToken: string;
  onAccountDeleted: () => Promise<void>;
  onSessionExpired: () => Promise<void>;
};

function downloadJson(payload: unknown) {
  const blob = new Blob([`${JSON.stringify(payload, null, 2)}\n`], {
    type: "application/json",
  });
  const objectUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.download = "issuescout-account-export.json";
  link.href = objectUrl;
  link.click();
  URL.revokeObjectURL(objectUrl);
}

export function PrivacyPanel({
  csrfToken,
  onAccountDeleted,
  onSessionExpired,
}: Props) {
  const [confirmation, setConfirmation] = useState("");
  const exportMutation = useMutation({
    mutationFn: () => exportAccount(),
    onError(error) {
      if (error instanceof ApiError && error.status === 401) {
        void onSessionExpired();
      }
    },
    onSuccess(envelope) {
      downloadJson(envelope.data);
    },
  });
  const deleteMutation = useMutation({
    mutationFn: () => deleteAccount(csrfToken),
    async onError(error) {
      if (error instanceof ApiError && error.status === 401) {
        await onSessionExpired();
      }
    },
    async onSuccess() {
      await onAccountDeleted();
    },
  });

  return (
    <section aria-labelledby="privacy-heading" className="grid gap-5">
      <Card>
        <CardHeader>
          <CardTitle id="privacy-heading">Privacy and account data</CardTitle>
          <CardDescription>
            Export exactly the account-owned feature data IssueScout retains, or
            permanently remove the account and all linked records.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5">
          <div className="rounded-xl border border-border bg-muted/30 p-5">
            <h3 className="font-semibold">Portable JSON export</h3>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
              Includes bookmarks, validated saved filters, and preferences.
              Sessions, credential hashes, OAuth state, GitHub payloads, and
              anonymous activity are excluded.
            </p>
            <Button
              className="mt-4"
              disabled={exportMutation.isPending}
              onClick={() => exportMutation.mutate()}
              variant="outline"
            >
              {exportMutation.isPending ? "Preparing…" : "Download export"}
            </Button>
          </div>

          <div className="rounded-xl border border-danger/25 bg-danger-soft p-5">
            <h3 className="font-semibold text-danger">
              Permanently delete account
            </h3>
            <p className="mt-2 max-w-2xl text-sm leading-6 text-foreground/80">
              This cascades through identities, sessions, bookmarks, saved
              searches, and preferences. It cannot be undone.
            </p>
            <Dialog>
              <DialogTrigger asChild>
                <Button className="mt-4" variant="danger">
                  Delete account
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Delete your IssueScout account?</DialogTitle>
                  <DialogDescription>
                    Enter DELETE exactly. The server validates the confirmation
                    and revokes every active session.
                  </DialogDescription>
                </DialogHeader>
                <Field
                  description="This action is permanent."
                  htmlFor="account-delete-confirmation"
                  label="Type DELETE to confirm"
                >
                  <Input
                    autoComplete="off"
                    id="account-delete-confirmation"
                    onChange={(event) => setConfirmation(event.target.value)}
                    value={confirmation}
                  />
                </Field>
                {deleteMutation.error ? (
                  <AccountRequestAlert error={deleteMutation.error} />
                ) : null}
                <div className="flex flex-wrap justify-end gap-3">
                  <DialogClose asChild>
                    <Button variant="outline">Cancel</Button>
                  </DialogClose>
                  <Button
                    disabled={
                      confirmation !== "DELETE" || deleteMutation.isPending
                    }
                    onClick={() => deleteMutation.mutate()}
                    variant="danger"
                  >
                    {deleteMutation.isPending
                      ? "Deleting account…"
                      : "Delete permanently"}
                  </Button>
                </div>
              </DialogContent>
            </Dialog>
          </div>
        </CardContent>
      </Card>
      {exportMutation.error ? (
        <AccountRequestAlert error={exportMutation.error} />
      ) : null}
    </section>
  );
}
