import { CircleOff, Gauge, SearchCheck } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../../components/ui/alert";
import { Button } from "../../../components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import { Pagination } from "../../../components/ui/pagination";
import type { RepositoryDiscoveryEnvelope } from "../../../shared/api/generated";
import { formatCompactNumber } from "../../../shared/lib/format";
import { RepositoryCard } from "./RepositoryCard";

type RepositoryDiscoveryResultsProps = {
  envelope: RepositoryDiscoveryEnvelope;
  isFetching: boolean;
  onPageChange: (page: number) => void;
};

export function RepositoryDiscoveryResults({
  envelope,
  isFetching,
  onPageChange,
}: RepositoryDiscoveryResultsProps) {
  const { items, pagination, searchSummary, warnings } = envelope.data;
  if (items.length === 0) {
    const outOfRange =
      pagination.total > 0 &&
      pagination.totalPages > 0 &&
      pagination.page > pagination.totalPages;
    return (
      <Card>
        <CardContent className="grid justify-items-center gap-4 p-8 text-center sm:p-12">
          <span className="grid size-14 place-items-center rounded-2xl bg-muted text-muted-foreground">
            <Icon className="size-6" icon={CircleOff} />
          </span>
          <div>
            <h2 className="text-xl font-semibold">
              {outOfRange
                ? "This repository page is no longer available"
                : "No eligible repositories found"}
            </h2>
            <p className="mx-auto mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
              {outOfRange
                ? "The bounded result set changed after this URL was shared. Return to the first server-ordered page."
                : "GitHub candidates were checked, but none met every condition. Try fewer technology terms, a lower readiness threshold, or a broader recency window."}
            </p>
          </div>
          {outOfRange ? (
            <Button onClick={() => onPageChange(1)}>Return to page 1</Button>
          ) : (
            <a
              className="rounded-lg text-sm font-semibold text-accent outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
              href="#repository-filters"
            >
              Broaden the filters
            </a>
          )}
        </CardContent>
      </Card>
    );
  }

  const hasPartialEvidence =
    warnings.length > 0 ||
    searchSummary.enrichmentFailed > 0 ||
    searchSummary.enrichmentIncomplete ||
    searchSummary.githubIncomplete;
  const firstRank = (pagination.page - 1) * pagination.perPage + 1;

  return (
    <section
      aria-labelledby="repository-results-heading"
      className="grid gap-5"
    >
      <Card>
        <CardHeader className="sm:flex sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="font-mono text-xs tracking-[0.14em] text-accent uppercase">
              Server-ordered repositories
            </p>
            <CardTitle
              className="mt-2 text-2xl"
              id="repository-results-heading"
            >
              {formatCompactNumber(pagination.total)} eligible repositories
            </CardTitle>
          </div>
          <span className="inline-flex items-center gap-2 self-start rounded-full border border-border bg-muted px-3 py-2 text-xs text-muted-foreground">
            <Icon icon={Gauge} />
            {formatCompactNumber(searchSummary.candidatesChecked)} checked ·{" "}
            {formatCompactNumber(searchSummary.enrichmentAttempted)} enriched
          </span>
        </CardHeader>
        <CardContent className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2">
          <p>
            GitHub reported{" "}
            <strong className="text-foreground">
              {formatCompactNumber(searchSummary.upstreamTotal)}
            </strong>{" "}
            matching repositories before the bounded window.
          </p>
          {envelope.meta.rateLimitRemaining !== undefined ? (
            <p className="sm:text-right">
              GitHub API requests remaining:{" "}
              <strong className="text-foreground">
                {formatCompactNumber(envelope.meta.rateLimitRemaining)}
              </strong>
            </p>
          ) : null}
        </CardContent>
      </Card>

      {hasPartialEvidence ? (
        <Alert variant="warning">
          <AlertTitle>Some repository evidence is partial</AlertTitle>
          <AlertDescription>
            Eligible results keep deterministic server ordering, but optional
            GitHub evidence was incomplete.{" "}
            {warnings.map((warning) => warning.message).join(" ")}
          </AlertDescription>
        </Alert>
      ) : (
        <Alert variant="success">
          <AlertTitle>Bounded analysis completed</AlertTitle>
          <AlertDescription>
            <span className="inline-flex items-center gap-2">
              <Icon icon={SearchCheck} />
              Cards preserve the exact server order and explain every
              repository-level decision.
            </span>
          </AlertDescription>
        </Alert>
      )}

      <ol className="grid gap-5">
        {items.map((item, index) => (
          <li key={item.repository.fullName}>
            <RepositoryCard item={item} rank={firstRank + index} />
          </li>
        ))}
      </ol>

      <Card>
        <CardContent className="p-5 sm:p-6">
          <Pagination
            ariaLabel="Repository discovery pagination"
            disabled={isFetching}
            hasNext={pagination.hasNext}
            onPageChange={onPageChange}
            page={pagination.page}
            totalPages={pagination.totalPages}
          />
        </CardContent>
      </Card>
    </section>
  );
}
