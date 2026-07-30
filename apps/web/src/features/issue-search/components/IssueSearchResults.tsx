import { CircleOff, Gauge, SearchCheck } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../../components/ui/alert";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import { Pagination } from "../../../components/ui/pagination";
import type { IssueSearchEnvelope } from "../../../shared/api/generated";
import { formatCompactNumber } from "../../../shared/lib/format";
import { RecommendationCard } from "./RecommendationCard";

type IssueSearchResultsProps = {
  envelope: IssueSearchEnvelope;
  isFetching: boolean;
  onPageChange: (page: number) => void;
};

export function IssueSearchResults({
  envelope,
  isFetching,
  onPageChange,
}: IssueSearchResultsProps) {
  const { items, pagination, searchSummary, warnings } = envelope.data;
  if (items.length === 0) {
    return (
      <Card>
        <CardContent className="grid justify-items-center gap-4 p-8 text-center sm:p-12">
          <span className="grid size-14 place-items-center rounded-2xl bg-muted text-muted-foreground">
            <Icon className="size-6" icon={CircleOff} />
          </span>
          <div>
            <h2 className="text-xl font-semibold">No eligible issues found</h2>
            <p className="mx-auto mt-2 max-w-xl text-sm leading-6 text-muted-foreground">
              GitHub candidates were checked, but none met every validated
              condition. Try fewer framework terms, a lower star threshold, or
              more available time.
            </p>
          </div>
          <a
            className="rounded-lg text-sm font-semibold text-accent outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
            href="#search-filters"
          >
            Broaden the filters
          </a>
        </CardContent>
      </Card>
    );
  }

  const hasPartialEvidence =
    warnings.length > 0 || searchSummary.enrichmentFailed > 0;
  const firstRank = (pagination.page - 1) * pagination.perPage + 1;
  return (
    <section aria-labelledby="issue-results-heading" className="grid gap-5">
      <Card>
        <CardHeader className="sm:flex sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="font-mono text-xs tracking-[0.14em] text-accent uppercase">
              Server-ranked recommendations
            </p>
            <CardTitle className="mt-2 text-2xl" id="issue-results-heading">
              {formatCompactNumber(pagination.total)} eligible issues
            </CardTitle>
          </div>
          <span className="inline-flex items-center gap-2 self-start rounded-full border border-border bg-muted px-3 py-2 text-xs text-muted-foreground">
            <Icon icon={Gauge} />
            {formatCompactNumber(searchSummary.candidatesChecked)} checked ·{" "}
            {formatCompactNumber(searchSummary.enrichmentAttempted)} enriched
          </span>
        </CardHeader>
        {envelope.meta.rateLimitRemaining !== undefined ? (
          <CardContent>
            <p className="text-xs text-muted-foreground">
              GitHub API requests remaining:{" "}
              <strong className="text-foreground">
                {formatCompactNumber(envelope.meta.rateLimitRemaining)}
              </strong>
            </p>
          </CardContent>
        ) : null}
      </Card>

      {hasPartialEvidence ? (
        <Alert variant="warning">
          <AlertTitle>Some recommendation evidence is partial</AlertTitle>
          <AlertDescription>
            Ranking remains deterministic, but GitHub omitted optional evidence
            for part of this bounded window.{" "}
            {warnings.map((warning) => warning.message).join(" ")}
          </AlertDescription>
        </Alert>
      ) : (
        <Alert variant="success">
          <AlertTitle>Bounded analysis completed</AlertTitle>
          <AlertDescription>
            <span className="inline-flex items-center gap-2">
              <Icon icon={SearchCheck} />
              Cards below preserve the exact order returned by the API.
            </span>
          </AlertDescription>
        </Alert>
      )}

      <ol className="grid gap-5">
        {items.map((item, index) => {
          const rank = firstRank + index;
          return (
            <li key={`${item.repository.fullName}#${item.issue.number}`}>
              <RecommendationCard item={item} rank={rank} />
            </li>
          );
        })}
      </ol>

      <Card>
        <CardContent className="p-5 sm:p-6">
          <Pagination
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
