import { useSearchParams } from "react-router";

import { Badge } from "../../../components/ui/badge";
import { Button } from "../../../components/ui/button";
import type { AuthUser } from "../../../shared/api/generated";
import { cn } from "../../../shared/lib/cn";
import { BookmarksPanel } from "./BookmarksPanel";
import { PreferencesPanel } from "./PreferencesPanel";
import { PrivacyPanel } from "./PrivacyPanel";
import { SavedSearchesPanel } from "./SavedSearchesPanel";

type Props = {
  csrfToken: string;
  onAccountDeleted: () => Promise<void>;
  onSessionExpired: () => Promise<void>;
  user: AuthUser;
};

const tabs = [
  { label: "Bookmarks", value: "bookmarks" },
  { label: "Saved searches", value: "saved" },
  { label: "Preferences", value: "preferences" },
  { label: "Privacy", value: "privacy" },
] as const;

type WorkspaceTab = (typeof tabs)[number]["value"];

function readTab(parameters: URLSearchParams): WorkspaceTab {
  const value = parameters.get("tab");
  return tabs.some((tab) => tab.value === value)
    ? (value as WorkspaceTab)
    : "bookmarks";
}

export function WorkspaceDashboard({
  csrfToken,
  onAccountDeleted,
  onSessionExpired,
  user,
}: Props) {
  const [parameters, setParameters] = useSearchParams();
  const activeTab = readTab(parameters);

  return (
    <div className="mx-auto min-h-[68vh] w-full max-w-7xl px-5 py-10 sm:px-8 sm:py-14 lg:px-10">
      <header className="flex flex-wrap items-end justify-between gap-6">
        <div className="max-w-3xl">
          <Badge variant="accent">Optional authenticated workspace</Badge>
          <h1 className="mt-5 text-4xl font-semibold tracking-[-0.055em] text-balance sm:text-5xl">
            Your saved IssueScout workspace.
          </h1>
          <p className="mt-4 max-w-2xl text-base leading-7 text-muted-foreground">
            Signed in as{" "}
            <strong className="text-foreground">{user.login}</strong>. Public
            discovery remains anonymous; only the features below use account
            storage.
          </p>
        </div>
      </header>

      <div className="mt-8 overflow-x-auto pb-2">
        <div
          aria-label="Account workspace sections"
          className="flex min-w-max gap-2"
          role="tablist"
        >
          {tabs.map((tab) => (
            <Button
              aria-controls={`workspace-panel-${tab.value}`}
              aria-selected={activeTab === tab.value}
              className={cn(
                activeTab === tab.value && "border-accent bg-accent-soft",
              )}
              id={`workspace-tab-${tab.value}`}
              key={tab.value}
              onClick={() => {
                const next = new URLSearchParams(parameters);
                next.set("tab", tab.value);
                setParameters(next);
              }}
              role="tab"
              variant="outline"
            >
              {tab.label}
            </Button>
          ))}
        </div>
      </div>

      <div
        aria-labelledby={`workspace-tab-${activeTab}`}
        className="mt-6"
        id={`workspace-panel-${activeTab}`}
        role="tabpanel"
      >
        {activeTab === "bookmarks" ? (
          <BookmarksPanel
            csrfToken={csrfToken}
            onSessionExpired={onSessionExpired}
          />
        ) : null}
        {activeTab === "saved" ? (
          <SavedSearchesPanel
            csrfToken={csrfToken}
            onSessionExpired={onSessionExpired}
          />
        ) : null}
        {activeTab === "preferences" ? (
          <PreferencesPanel
            csrfToken={csrfToken}
            onSessionExpired={onSessionExpired}
          />
        ) : null}
        {activeTab === "privacy" ? (
          <PrivacyPanel
            csrfToken={csrfToken}
            onAccountDeleted={onAccountDeleted}
            onSessionExpired={onSessionExpired}
          />
        ) : null}
      </div>
    </div>
  );
}
