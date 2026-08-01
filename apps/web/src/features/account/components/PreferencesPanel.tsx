import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState, type FormEvent } from "react";

import { Button } from "../../../components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "../../../components/ui/card";
import { Field } from "../../../components/ui/field";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import { ApiError } from "../../../shared/api/client";
import type {
  Preferences,
  PreferencesWriteRequest,
  ReducedMotionPreference,
  ThemePreference,
} from "../../../shared/api/generated";
import { queryKeys } from "../../../shared/query/query-keys";
import { getPreferences, updatePreferences } from "../api/account";
import { applyPreferences, preferenceOptions } from "../model/preferences";
import { AccountRequestAlert } from "./AccountRequestAlert";

type Props = {
  csrfToken: string;
  onSessionExpired: () => Promise<void>;
};

const defaults: Preferences = {
  reducedMotion: "system",
  resultsPerPage: 20,
  theme: "system",
  version: 0,
};

function PreferencesForm({
  disabled,
  onSubmit,
  preferences,
}: {
  disabled: boolean;
  onSubmit: (request: PreferencesWriteRequest) => void;
  preferences: Preferences;
}) {
  const [theme, setTheme] = useState<ThemePreference>(preferences.theme);
  const [reducedMotion, setReducedMotion] = useState<ReducedMotionPreference>(
    preferences.reducedMotion,
  );
  const [resultsPerPage, setResultsPerPage] = useState<10 | 20 | 50>(
    preferences.resultsPerPage,
  );

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit({
      reducedMotion,
      resultsPerPage,
      theme,
      version: preferences.version,
    });
  }

  return (
    <form className="grid gap-5 md:grid-cols-3" onSubmit={submit}>
      <Field htmlFor="preference-theme" label="Theme">
        <Select
          onValueChange={(value) => setTheme(value as ThemePreference)}
          value={theme}
        >
          <SelectTrigger id="preference-theme">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {preferenceOptions.theme.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <Field htmlFor="preference-motion" label="Motion">
        <Select
          onValueChange={(value) =>
            setReducedMotion(value as ReducedMotionPreference)
          }
          value={reducedMotion}
        >
          <SelectTrigger id="preference-motion">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {preferenceOptions.reducedMotion.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <Field htmlFor="preference-page-size" label="Results per page">
        <Select
          onValueChange={(value) =>
            setResultsPerPage(Number(value) as 10 | 20 | 50)
          }
          value={resultsPerPage.toString()}
        >
          <SelectTrigger id="preference-page-size">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {preferenceOptions.resultsPerPage.map((value) => (
              <SelectItem key={value} value={value.toString()}>
                {value} results
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>
      <div className="md:col-span-3">
        <Button disabled={disabled} type="submit">
          {disabled ? "Saving…" : "Save preferences"}
        </Button>
      </div>
    </form>
  );
}

export function PreferencesPanel({ csrfToken, onSessionExpired }: Props) {
  const queryClient = useQueryClient();
  const query = useQuery({
    queryFn: ({ signal }) => getPreferences(signal),
    queryKey: queryKeys.account.preferences,
  });

  useEffect(() => {
    if (query.data) {
      applyPreferences(query.data.data);
    }
  }, [query.data]);

  const update = useMutation({
    mutationFn: (request: PreferencesWriteRequest) =>
      updatePreferences(request, csrfToken),
    async onError(error) {
      if (error instanceof ApiError && error.status === 401) {
        await onSessionExpired();
      }
    },
    onSuccess(envelope) {
      queryClient.setQueryData(queryKeys.account.preferences, envelope);
      applyPreferences(envelope.data);
    },
  });

  return (
    <section aria-labelledby="preferences-heading" className="grid gap-5">
      <Card>
        <CardHeader>
          <CardTitle id="preferences-heading">Display preferences</CardTitle>
          <CardDescription>
            Preferences are account-owned and versioned. They never alter
            anonymous users or require browser storage.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {query.isPending ? (
            <p className="text-sm text-muted-foreground" role="status">
              Loading preferences…
            </p>
          ) : (
            <PreferencesForm
              disabled={update.isPending}
              key={query.data?.data.version ?? 0}
              onSubmit={(request) => update.mutate(request)}
              preferences={query.data?.data ?? defaults}
            />
          )}
        </CardContent>
      </Card>
      {query.error ? <AccountRequestAlert error={query.error} /> : null}
      {update.error ? <AccountRequestAlert error={update.error} /> : null}
    </section>
  );
}
