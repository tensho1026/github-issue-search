import { BookOpenCheck, SlidersHorizontal } from "lucide-react";
import { useMemo } from "react";
import { useSearchParams } from "react-router";

import { Badge } from "../components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "../components/ui/card";
import { Icon } from "../components/ui/icon";
import { useRepositoryDiscovery } from "../features/repository-discovery/api/useRepositoryDiscovery";
import { RepositoryDiscoveryForm } from "../features/repository-discovery/components/RepositoryDiscoveryForm";
import { RepositoryDiscoveryResults } from "../features/repository-discovery/components/RepositoryDiscoveryResults";
import {
  RepositoryDiscoveryBeforeState,
  RepositoryDiscoveryErrorState,
  RepositoryDiscoveryInvalidState,
  RepositoryDiscoveryLoadingState,
} from "../features/repository-discovery/components/RepositoryDiscoveryState";
import {
  decodeRepositorySearchParams,
  encodeRepositorySearchParams,
  type RepositoryFilters,
} from "../features/repository-discovery/model/repository-filters";

export function RepositoryDiscoveryPage() {
  const [searchParameters, setSearchParameters] = useSearchParams();
  const serializedSearch = searchParameters.toString();
  const location = useMemo(
    () => decodeRepositorySearchParams(new URLSearchParams(serializedSearch)),
    [serializedSearch],
  );
  const query = useRepositoryDiscovery(location);

  function submit(filters: RepositoryFilters) {
    setSearchParameters(encodeRepositorySearchParams(filters));
  }

  function changePage(page: number) {
    setSearchParameters(
      encodeRepositorySearchParams({
        ...location.filters,
        page,
      }),
    );
  }

  let resultContent;
  if (location.shouldSearch && !location.valid) {
    resultContent = <RepositoryDiscoveryInvalidState />;
  } else if (!location.shouldSearch) {
    resultContent = <RepositoryDiscoveryBeforeState />;
  } else if (query.isPending) {
    resultContent = <RepositoryDiscoveryLoadingState />;
  } else if (query.error) {
    resultContent = (
      <RepositoryDiscoveryErrorState
        error={query.error}
        isFetching={query.isFetching}
        onRetry={() => {
          void query.refetch();
        }}
      />
    );
  } else if (query.data) {
    resultContent = (
      <RepositoryDiscoveryResults
        envelope={query.data}
        isFetching={query.isFetching}
        onPageChange={changePage}
      />
    );
  } else {
    resultContent = <RepositoryDiscoveryLoadingState />;
  }

  return (
    <div className="mx-auto w-full max-w-7xl px-5 py-10 sm:px-8 sm:py-14 lg:px-10">
      <header className="max-w-3xl">
        <Badge variant="accent">
          <Icon icon={BookOpenCheck} />
          Anonymous OSS discovery
        </Badge>
        <h1 className="mt-5 text-4xl font-semibold tracking-[-0.055em] text-balance sm:text-5xl">
          Find a repository ready for your contribution.
        </h1>
        <p className="mt-5 max-w-2xl text-base leading-7 text-muted-foreground sm:text-lg">
          Filter public repositories by technology, license, activity, and
          contribution readiness. IssueScout explains bounded evidence without
          using an account or database.
        </p>
      </header>

      <div className="mt-9 grid items-start gap-6 xl:grid-cols-[minmax(24rem,0.86fr)_minmax(0,1.14fr)]">
        <Card
          className="overflow-hidden xl:sticky xl:top-24"
          id="repository-filters"
        >
          <CardHeader className="border-b border-border bg-muted/35">
            <span className="grid size-11 place-items-center rounded-xl bg-accent-soft text-accent-soft-foreground">
              <Icon className="size-5" icon={SlidersHorizontal} />
            </span>
            <CardTitle className="mt-2">Repository criteria</CardTitle>
            <CardDescription>
              Validated filters and page state are shareable in the URL. Result
              payloads and anonymous search history are never stored.
            </CardDescription>
          </CardHeader>
          <CardContent className="p-5 sm:p-6">
            <RepositoryDiscoveryForm
              defaultValues={location.filters}
              disabled={query.isFetching}
              locationErrors={
                location.shouldSearch ? location.errors : undefined
              }
              onSubmit={submit}
            />
          </CardContent>
        </Card>

        <div aria-live="polite" aria-relevant="additions text">
          {resultContent}
        </div>
      </div>
    </div>
  );
}
