import { ArrowLeft } from "lucide-react";
import { useEffect, useRef, type ReactNode } from "react";
import { Link } from "react-router";

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../../components/ui/alert";
import { Badge } from "../../../components/ui/badge";
import { Button } from "../../../components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import type {
  CountAggregate,
  DurationAggregate,
  Evidence,
  IssueDetailEnvelope,
  RatioAggregate,
} from "../../../shared/api/generated";
import { externalLinks } from "../../../shared/config/app-config";
import { cn } from "../../../shared/lib/cn";
import {
  formatCompactNumber,
  formatDate,
  formatDuration,
  formatPercentage,
} from "../../../shared/lib/format";
import {
  scorePresentation,
  skillPresentation,
  warningPresentation,
} from "../../issue-search/model/search-presentation";
import { BookmarkAction } from "../../account/components/BookmarkAction";
import {
  categoryLabel,
  qualitySignalLabel,
  repositorySignalLabel,
  scopeAreaLabel,
  scoreComponentLabel,
  signalPresentation,
} from "../model/detail-presentation";
import { SafeIssueBody } from "./SafeIssueBody";

type Props = {
  envelope: IssueDetailEnvelope;
  returnTo: string;
};

type Sample = Pick<
  CountAggregate,
  "confidence" | "sampleSize" | "truncated" | "windowDays"
>;

function Section({ children, title }: { children: ReactNode; title: string }) {
  return (
    <Card className="min-w-0 overflow-hidden">
      <CardHeader className="border-b border-border bg-muted/25">
        <CardTitle>{title}</CardTitle>
      </CardHeader>
      <CardContent className="p-5 sm:p-6">{children}</CardContent>
    </Card>
  );
}

function EvidenceList({ items }: { items: Evidence[] }) {
  return items.length > 0 ? (
    <ul className="mt-2 grid gap-1 text-xs leading-5 text-muted-foreground">
      {items.map((item) => (
        <li key={`${item.ruleId}-${item.source}-${item.description}`}>
          {item.description}
        </li>
      ))}
    </ul>
  ) : null;
}

function Facts({ items }: { items: Array<[string, ReactNode]> }) {
  return (
    <dl className="grid grid-cols-2 gap-4">
      {items.map(([label, value]) => (
        <div key={label}>
          <dt className="text-xs text-muted-foreground">{label}</dt>
          <dd className="mt-1 font-medium">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

function Metric({
  detail,
  label,
  sample,
  value,
}: {
  detail?: string;
  label: string;
  sample: Sample;
  value: string;
}) {
  return (
    <div className="rounded-xl border border-border bg-muted/25 p-3">
      <dt className="text-xs font-semibold text-muted-foreground uppercase">
        {label}
      </dt>
      <dd className="mt-1 font-mono text-xl font-semibold">{value}</dd>
      {detail ? (
        <p className="text-xs text-muted-foreground">{detail}</p>
      ) : null}
      <p className="mt-1 text-xs leading-5 text-muted-foreground">
        {sample.windowDays}d · {sample.sampleSize} sampled · {sample.confidence}
        {sample.truncated ? " · bounded" : ""}
      </p>
    </div>
  );
}

function countValue(metric: CountAggregate) {
  return metric.status === "available" && metric.value !== null
    ? formatCompactNumber(metric.value)
    : "Unavailable";
}

function durationValue(metric: DurationAggregate) {
  return metric.status === "available"
    ? formatDuration(metric.medianSeconds)
    : "Unavailable";
}

function ratioValue(metric: RatioAggregate) {
  return metric.status === "available" && metric.percentage !== null
    ? formatPercentage(metric.percentage)
    : "Unavailable";
}

export function IssueDetailDashboard({ envelope, returnTo }: Props) {
  const { data, meta } = envelope;
  const titleRef = useRef<HTMLHeadingElement>(null);
  const score = scorePresentation(data.recommendation.score);
  const activityMetrics: Array<{
    detail?: string;
    label: string;
    sample: Sample;
    value: string;
  }> = [
    {
      label: "Contributors",
      sample: data.activity.contributors,
      value: countValue(data.activity.contributors),
    },
    {
      label: "Pull requests opened",
      sample: data.activity.pullRequestsOpened,
      value: countValue(data.activity.pullRequestsOpened),
    },
    {
      label: "Stale open pull requests",
      sample: data.activity.staleOpenPullRequests,
      value: countValue(data.activity.staleOpenPullRequests),
    },
    {
      label: "Unanswered issues",
      sample: data.activity.unansweredIssues,
      value: countValue(data.activity.unansweredIssues),
    },
    {
      detail: `${data.activity.pullRequestMerge.numerator ?? "—"} of ${data.activity.pullRequestMerge.denominator ?? "—"} opened`,
      label: "Pull request merge rate",
      sample: data.activity.pullRequestMerge,
      value: ratioValue(data.activity.pullRequestMerge),
    },
    ...(
      [
        ["First issue response", data.activity.issueResponse],
        ["First pull request review", data.activity.pullRequestReview],
        ["Pull request merge time", data.activity.pullRequestMergeTime],
      ] as Array<[string, DurationAggregate]>
    ).map(([label, metric]) => ({
      detail: `90th percentile: ${
        metric.status === "available"
          ? formatDuration(metric.percentile90Seconds)
          : "Unavailable"
      }`,
      label,
      sample: metric,
      value: durationValue(metric),
    })),
  ];

  useEffect(() => {
    titleRef.current?.focus({ preventScroll: true });
  }, []);

  return (
    <>
      <Link
        className="inline-flex min-h-11 items-center gap-2 rounded-lg text-sm font-semibold text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
        to={returnTo}
      >
        <Icon icon={ArrowLeft} />
        Back to search results
      </Link>

      {data.inspection.incomplete ? (
        <Alert className="mt-4" variant="warning">
          <AlertTitle>Partial GitHub inspection</AlertTitle>
          <AlertDescription>
            Optional samples were incomplete. Unknown and unavailable values
            preserve that uncertainty instead of implying zero.
          </AlertDescription>
        </Alert>
      ) : null}

      <Card className="mt-4 overflow-hidden">
        <CardHeader className="border-b border-border bg-muted/30 p-6 sm:p-8">
          <div className="flex flex-wrap items-start justify-between gap-6">
            <div className="min-w-0 flex-1">
              <p className="text-xs text-muted-foreground">
                <a
                  className="rounded font-mono outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
                  href={externalLinks.gitHubRepository(
                    data.repository.owner,
                    data.repository.name,
                  )}
                  rel="noreferrer"
                  target="_blank"
                >
                  {data.repository.fullName}
                </a>{" "}
                · Issue #{data.issue.number} · {data.issue.state}
              </p>
              <h1
                className="mt-4 max-w-4xl rounded text-3xl font-semibold tracking-[-0.05em] outline-none sm:text-5xl"
                ref={titleRef}
                tabIndex={-1}
              >
                {data.issue.title}
              </h1>
              <p className="mt-4 max-w-3xl leading-7 text-muted-foreground">
                {data.repository.description ||
                  "No public repository description was provided."}
              </p>
            </div>
            <div
              aria-label={`${data.recommendation.score} out of 100, ${score.label}`}
              aria-valuemax={100}
              aria-valuemin={0}
              aria-valuenow={data.recommendation.score}
              className={cn(
                "grid size-24 shrink-0 place-items-center rounded-2xl border text-center",
                score.className,
              )}
              role="meter"
            >
              <span>
                <strong className="block font-mono text-3xl">
                  {data.recommendation.score}
                </strong>
                <small className="font-semibold uppercase">{score.label}</small>
              </span>
            </div>
          </div>
          <div className="mt-4 flex flex-wrap gap-2">
            <Badge variant="info">
              {data.repository.mainLanguage || "Language unknown"}
            </Badge>
            <Badge variant="neutral">
              {formatCompactNumber(data.repository.stars)} stars
            </Badge>
            <Badge variant="neutral">
              Difficulty {data.analysis.difficulty.level}:{" "}
              {data.analysis.difficulty.label}
            </Badge>
            <Badge variant="neutral">{data.analysis.effort.label}</Badge>
            {data.issue.locked ? (
              <Badge variant="warning">Conversation locked</Badge>
            ) : null}
            {data.repository.isArchived ? (
              <Badge variant="warning">Archived repository</Badge>
            ) : null}
          </div>
          <div className="mt-5">
            <BookmarkAction
              request={{
                issueNumber: data.issue.number,
                repositoryName: data.repository.name,
                repositoryOwner: data.repository.owner,
                targetType: "issue",
              }}
            />
          </div>
          <div className="mt-4 flex flex-wrap gap-3">
            <Button asChild>
              <a
                href={externalLinks.gitHubIssue(
                  data.repository.owner,
                  data.repository.name,
                  data.issue.number,
                )}
                rel="noreferrer"
                target="_blank"
              >
                Open original GitHub issue
              </a>
            </Button>
          </div>
        </CardHeader>
        <CardContent className="p-5 sm:p-6">
          <Facts
            items={[
              [
                "Author",
                <a
                  href={externalLinks.gitHubProfile(data.issue.author.login)}
                  key="author"
                  rel="noreferrer"
                  target="_blank"
                >
                  @{data.issue.author.login} · {data.issue.author.type}
                </a>,
              ],
              [
                "Conversation",
                `${formatCompactNumber(data.issue.comments)} comments`,
              ],
              [
                "Assignees",
                data.issue.assignees.length > 0
                  ? data.issue.assignees
                      .map((assignee) => `@${assignee}`)
                      .join(", ")
                  : "Unassigned",
              ],
              [
                "Dates",
                `${formatDate(data.issue.createdAt)} → ${formatDate(
                  data.issue.updatedAt,
                )}`,
              ],
            ]}
          />
          <div className="mt-4 flex flex-wrap gap-2">
            {data.issue.labels.length > 0 ? (
              data.issue.labels.map((label) => (
                <Badge key={label} variant="accent">
                  {label}
                </Badge>
              ))
            ) : (
              <span className="text-sm text-muted-foreground">
                No public labels
              </span>
            )}
          </div>
        </CardContent>
      </Card>

      <div className="mt-6 grid items-start gap-6 lg:grid-cols-[minmax(0,1.28fr)_minmax(19rem,0.72fr)]">
        <div className="grid min-w-0 gap-6">
          <Section title="Issue description">
            <SafeIssueBody body={data.issue.body} />
          </Section>

          <Section title="What this work involves">
            <Facts
              items={[
                [
                  "Category",
                  data.analysis.category.matches.map(categoryLabel).join(", "),
                ],
                [
                  "Difficulty",
                  `${data.analysis.difficulty.label} · ${data.analysis.difficulty.confidence}`,
                ],
                [
                  "Effort",
                  `${data.analysis.effort.label} · ${data.analysis.effort.confidence}`,
                ],
                [
                  "Scope",
                  `${data.analysis.scope.fileCount.label} · ${data.analysis.scope.confidence}`,
                ],
              ]}
            />
            <div className="mt-4 flex flex-wrap gap-2">
              {data.analysis.scope.areas.map((area) => (
                <Badge key={area} variant="info">
                  {scopeAreaLabel(area)}
                </Badge>
              ))}
              <Badge
                variant={
                  signalPresentation(data.analysis.scope.databaseChange).tone
                }
              >
                Database:{" "}
                {signalPresentation(data.analysis.scope.databaseChange).label}
              </Badge>
              <Badge variant="neutral">
                Analysis: {data.analysis.confidence} confidence
              </Badge>
            </div>
            <EvidenceList items={data.analysis.scope.evidence} />

            <h3 className="mt-6 font-semibold">Required technologies</h3>
            <ul className="mt-3 grid gap-3 sm:grid-cols-2">
              {data.analysis.requiredTechnologies.map((technology) => (
                <li
                  className="rounded-xl border border-border bg-muted/25 p-3"
                  key={`${technology.kind}-${technology.name}`}
                >
                  <strong>{technology.name}</strong>{" "}
                  <Badge variant="neutral">{technology.kind}</Badge>
                  <p className="text-xs text-muted-foreground">
                    {technology.confidence} confidence
                  </p>
                  <EvidenceList items={technology.evidence} />
                </li>
              ))}
            </ul>

            <h3 className="mt-6 font-semibold">
              Issue quality · {data.analysis.quality.score}/100 ·{" "}
              {data.analysis.quality.confidence} confidence
            </h3>
            <ul className="mt-3 grid gap-2 sm:grid-cols-2">
              {data.analysis.quality.signals.map((signal) => {
                const state = signalPresentation(signal.state);
                return (
                  <li
                    className="rounded-xl border border-border bg-muted/25 p-3"
                    key={signal.key}
                  >
                    <span className="text-sm font-medium">
                      {qualitySignalLabel(signal.key)}
                    </span>{" "}
                    <Badge variant={state.tone}>{state.label}</Badge>
                    <EvidenceList items={signal.evidence} />
                  </li>
                );
              })}
            </ul>
            <h3 className="mt-6 font-semibold">Assessment evidence</h3>
            <EvidenceList
              items={[
                ...data.analysis.category.evidence,
                ...data.analysis.difficulty.evidence,
                ...data.analysis.effort.evidence,
              ]}
            />
          </Section>

          <Section title="Why IssueScout recommends it">
            <p className="text-xl font-semibold">
              Skill match: {data.recommendation.skillMatch.matched}/
              {data.recommendation.skillMatch.denominator} ·{" "}
              {formatPercentage(data.recommendation.skillMatch.percentage)}
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              {data.recommendation.skillMatch.skills.length > 0
                ? data.recommendation.skillMatch.skills.map((skill) => (
                    <Badge
                      key={`${skill.technology}-${skill.status}`}
                      variant={skillPresentation(skill.status)}
                    >
                      {skill.technology}: {skill.status}
                    </Badge>
                  ))
                : "No comparable skill evidence"}
            </div>
            <ul className="mt-4 list-disc space-y-1 pl-5 text-sm">
              {data.recommendation.reasons.map((reason) => (
                <li key={reason}>{reason}</li>
              ))}
            </ul>

            <h3 className="mt-6 font-semibold">100-point score breakdown</h3>
            <ul className="mt-3 grid gap-3">
              {data.recommendation.breakdown.map((component) => (
                <li
                  className="rounded-xl border border-border bg-muted/25 p-3"
                  key={component.name}
                >
                  <strong>
                    {scoreComponentLabel(component.name)} · {component.score}/
                    {component.maximum}
                  </strong>
                  <progress
                    aria-label={`${scoreComponentLabel(component.name)} score`}
                    className="mt-2 block h-2 w-full accent-accent"
                    max={component.maximum}
                    value={component.score}
                  />
                  <ul className="mt-2 text-xs text-muted-foreground">
                    {component.reasons.map((reason) => (
                      <li key={reason}>{reason}</li>
                    ))}
                  </ul>
                </li>
              ))}
            </ul>

            <div className="mt-5">
              <Badge
                variant={
                  data.recommendation.claim.claimed ? "warning" : "success"
                }
              >
                {data.recommendation.claim.claimed
                  ? "Possibly claimed"
                  : "No claim detected"}{" "}
                · {data.recommendation.claim.confidence} confidence
              </Badge>
              <EvidenceList items={data.recommendation.claim.evidence} />
            </div>
            <div className="mt-4 grid gap-2">
              {data.recommendation.warnings.map((warning) => (
                <Alert
                  key={`${warning.code}-${warning.message}`}
                  variant={warningPresentation(warning.severity)}
                >
                  <AlertTitle>{warning.code.replaceAll("_", " ")}</AlertTitle>
                  <AlertDescription>{warning.message}</AlertDescription>
                  <EvidenceList items={warning.evidence} />
                </Alert>
              ))}
            </div>
          </Section>
        </div>

        <aside
          aria-label="Repository and maintainer signals"
          className="grid gap-6"
        >
          <Section title="Repository snapshot">
            <Facts
              items={[
                ["Stars", formatCompactNumber(data.repository.stars)],
                ["Forks", formatCompactNumber(data.repository.forks)],
                [
                  "Open issues",
                  formatCompactNumber(data.repository.openIssues),
                ],
                ["Default branch", data.repository.defaultBranch],
                ["Repository ID", data.repository.id],
                [
                  "Kind",
                  data.repository.isFork
                    ? "Forked repository"
                    : "Source repository",
                ],
                ["Updated", formatDate(data.repository.updatedAt)],
                [
                  "Last pushed",
                  data.repository.pushedAt
                    ? formatDate(data.repository.pushedAt)
                    : "Unknown",
                ],
              ]}
            />
          </Section>

          <Section title="Contributor readiness">
            <ul className="grid gap-2">
              {data.repositoryHealth.map((signal) => {
                const state = signalPresentation(signal.state);
                return (
                  <li
                    className="rounded-xl border border-border bg-muted/25 p-3"
                    key={signal.key}
                  >
                    {repositorySignalLabel(signal.key)}{" "}
                    <Badge variant={state.tone}>{state.label}</Badge>
                    <EvidenceList items={signal.evidence} />
                  </li>
                );
              })}
            </ul>
          </Section>

          <Section title="Maintainer activity">
            <p className="mb-3 text-sm text-muted-foreground">
              CI: {data.activity.ci} · Meaningful update:{" "}
              {data.activity.lastMeaningfulUpdate
                ? formatDate(data.activity.lastMeaningfulUpdate)
                : "Unknown"}
            </p>
            <dl className="grid gap-2">
              {activityMetrics.map((metric) => (
                <Metric key={metric.label} {...metric} />
              ))}
            </dl>
          </Section>

          <Card>
            <CardContent className="grid gap-1 p-5 text-xs text-muted-foreground">
              <p>Generated {formatDate(meta.timestamp)}</p>
              <p>
                Request ID: <code>{meta.requestId}</code>
              </p>
              <p>
                Rate limit remaining:{" "}
                {meta.rateLimitRemaining === undefined
                  ? "Unavailable"
                  : formatCompactNumber(meta.rateLimitRemaining)}
              </p>
            </CardContent>
          </Card>
        </aside>
      </div>
    </>
  );
}
