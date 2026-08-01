import {
  ArrowUpRight,
  BookOpen,
  CheckCircle2,
  CircleMinus,
  Clock3,
  GitFork,
  Languages,
  MessageCircle,
  ShieldCheck,
  Star,
  Tag,
  Users,
} from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "../../../components/ui/alert";
import { Badge } from "../../../components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "../../../components/ui/tooltip";
import type { RepositoryDiscoveryItem } from "../../../shared/api/generated";
import { formatCompactNumber, formatDate } from "../../../shared/lib/format";
import {
  categoryLabel,
  confidenceLabel,
  difficultyPresentation,
  evidencePresentation,
  readinessPresentation,
} from "../model/repository-presentation";
import { BookmarkAction } from "../../account/components/BookmarkAction";

type RepositoryCardProps = {
  item: RepositoryDiscoveryItem;
  rank: number;
};

type SignalProps = {
  available: boolean;
  label: string;
};

function Signal({ available, label }: SignalProps) {
  return (
    <li className="flex items-center gap-2 text-sm">
      <Icon
        className={available ? "text-success" : "text-muted-foreground"}
        icon={available ? CheckCircle2 : CircleMinus}
      />
      <span>{label}</span>
    </li>
  );
}

export function RepositoryCard({ item, rank }: RepositoryCardProps) {
  const readiness = readinessPresentation(item.readiness);
  const difficulty = difficultyPresentation(item.difficulty);
  const documentation = evidencePresentation(item.documentation.status);
  const japanese = item.documentation.japaneseReadme;
  const japaneseEvidence = evidencePresentation(japanese.status);

  return (
    <Card className="overflow-hidden">
      <CardHeader className="border-b border-border bg-muted/25">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="neutral">#{rank}</Badge>
              <Badge variant="info">{categoryLabel(item.category)}</Badge>
              {item.repository.isFork ? (
                <Badge variant="neutral">Fork</Badge>
              ) : null}
              {item.repository.isArchived ? (
                <Badge variant="warning">Archived</Badge>
              ) : null}
            </div>
            <CardTitle className="mt-4 text-2xl">
              <a
                className="inline-flex max-w-full items-center gap-2 rounded-md outline-none hover:text-accent focus-visible:ring-2 focus-visible:ring-ring"
                href={item.repository.url}
                rel="noreferrer"
                target="_blank"
              >
                <span className="truncate">{item.repository.fullName}</span>
                <Icon className="shrink-0" icon={ArrowUpRight} />
              </a>
            </CardTitle>
            <CardDescription className="mt-2 max-w-3xl">
              {item.repository.description || "No public description."}
            </CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <Badge variant={readiness.tone}>
              {readiness.label} · {item.readiness.score}/100
            </Badge>
            <Badge variant={difficulty.tone}>
              Difficulty {item.difficulty.level}/5 · {difficulty.label}
            </Badge>
          </div>
        </div>
      </CardHeader>

      <CardContent className="grid gap-6 p-5 sm:p-6">
        <dl className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <Metric
            icon={Star}
            label="Stars"
            value={formatCompactNumber(item.popularity.stars)}
          />
          <Metric
            icon={GitFork}
            label="Forks"
            value={formatCompactNumber(item.popularity.forks)}
          />
          <Metric
            icon={Users}
            label="Watchers"
            value={formatCompactNumber(item.popularity.watchers)}
          />
          <Metric
            icon={MessageCircle}
            label="Open issues"
            value={formatCompactNumber(item.popularity.openIssues)}
          />
          <Metric
            icon={Clock3}
            label="Last push"
            value={formatDate(item.activity.pushedAt)}
          />
        </dl>

        <div className="flex flex-wrap gap-2">
          {item.language ? (
            <Badge variant="accent">
              <Icon icon={Languages} />
              {item.language}
            </Badge>
          ) : (
            <Badge variant="neutral">Language unavailable</Badge>
          )}
          <Badge variant={item.license.spdxId ? "success" : "neutral"}>
            <Icon icon={ShieldCheck} />
            {item.license.spdxId ?? "License unavailable"}
          </Badge>
          {item.technologies.map((technology) => (
            <Badge key={technology} variant="accent">
              {technology}
            </Badge>
          ))}
        </div>

        <div className="grid gap-5 xl:grid-cols-3">
          <section
            aria-labelledby={`${item.repository.fullName}-readiness`}
            className="rounded-xl border border-border bg-muted/30 p-4"
          >
            <h3
              className="font-semibold"
              id={`${item.repository.fullName}-readiness`}
            >
              Why this readiness
            </h3>
            <ul className="mt-3 grid gap-2 text-sm leading-6 text-muted-foreground">
              {item.readiness.reasons.map((reason) => (
                <li key={reason}>• {reason}</li>
              ))}
            </ul>
            <p className="mt-4 text-xs text-muted-foreground">
              {formatCompactNumber(item.readiness.goodFirstIssues)} good first
              issues · {formatCompactNumber(item.readiness.helpWantedIssues)}{" "}
              help wanted
            </p>
          </section>

          <section
            aria-labelledby={`${item.repository.fullName}-documentation`}
            className="rounded-xl border border-border bg-muted/30 p-4"
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3
                className="font-semibold"
                id={`${item.repository.fullName}-documentation`}
              >
                Contribution documents
              </h3>
              <Badge variant={documentation.tone}>{documentation.label}</Badge>
            </div>
            <ul className="mt-3 grid gap-2">
              <Signal
                available={item.documentation.readmeAvailable}
                label="README"
              />
              <Signal
                available={item.documentation.contributingGuide}
                label="Contributing guide"
              />
              <Signal
                available={item.documentation.codeOfConduct}
                label="Code of conduct"
              />
              <Signal
                available={item.documentation.securityPolicy}
                label="Security policy"
              />
            </ul>
          </section>

          <section
            aria-labelledby={`${item.repository.fullName}-japanese`}
            className="rounded-xl border border-border bg-muted/30 p-4"
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <h3
                className="font-semibold"
                id={`${item.repository.fullName}-japanese`}
              >
                Japanese README evidence
              </h3>
              <Badge variant={japaneseEvidence.tone}>
                {japaneseEvidence.label}
              </Badge>
            </div>
            <p className="mt-3 text-sm font-medium">
              {japanese.status === "unavailable"
                ? "Not analyzed"
                : japanese.detected
                  ? "Japanese script detected"
                  : "Japanese script not detected"}
            </p>
            <Tooltip>
              <TooltipTrigger asChild>
                <button
                  className="mt-2 rounded-md text-left text-xs text-muted-foreground underline decoration-dotted underline-offset-4 outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  type="button"
                >
                  {confidenceLabel(japanese.confidence)}
                </button>
              </TooltipTrigger>
              <TooltipContent className="max-w-xs">
                Heuristic only: at least 20 Japanese-script runes and 5% of
                analyzed letters are required for detection.
              </TooltipContent>
            </Tooltip>
            {japanese.status !== "unavailable" ? (
              <p className="mt-3 text-xs leading-5 text-muted-foreground">
                {formatCompactNumber(japanese.japaneseRunes)} Japanese-script
                runes across {formatCompactNumber(japanese.letterRunes)} letters
                in {formatCompactNumber(japanese.analyzedBytes)} analyzed bytes.
              </p>
            ) : null}
          </section>
        </div>

        <div className="grid gap-5 xl:grid-cols-2">
          <section aria-label="Difficulty reasons">
            <h3 className="font-semibold">Preliminary difficulty evidence</h3>
            <ul className="mt-2 grid gap-2 text-sm leading-6 text-muted-foreground">
              {item.difficulty.reasons.map((reason) => (
                <li key={reason}>• {reason}</li>
              ))}
            </ul>
          </section>
          <section aria-label="Repository topics">
            <h3 className="font-semibold">Public topics</h3>
            {item.topics.length > 0 ? (
              <ul className="mt-2 flex flex-wrap gap-2">
                {item.topics.map((topic) => (
                  <li key={topic}>
                    <Badge variant="neutral">
                      <Icon icon={Tag} />
                      {topic}
                    </Badge>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-2 text-sm text-muted-foreground">
                No public topics were returned.
              </p>
            )}
          </section>
        </div>

        {item.warnings.length > 0 ? (
          <Alert variant="warning">
            <AlertTitle>Repository evidence is partial</AlertTitle>
            <AlertDescription>
              {item.warnings.map((warning) => warning.message).join(" ")}
            </AlertDescription>
          </Alert>
        ) : null}

        <p className="flex items-center gap-2 text-xs text-muted-foreground">
          <Icon icon={BookOpen} />
          Repository updated {formatDate(item.activity.updatedAt)}. Server
          ordering and decisions are preserved.
        </p>
        <div>
          <BookmarkAction
            request={{
              repositoryName: item.repository.name,
              repositoryOwner: item.repository.owner,
              targetType: "repository",
            }}
          />
        </div>
      </CardContent>
    </Card>
  );
}

function Metric({
  icon,
  label,
  value,
}: {
  icon: typeof Star;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-xl bg-muted p-3">
      <dt className="flex items-center gap-2 text-xs text-muted-foreground">
        <Icon icon={icon} />
        {label}
      </dt>
      <dd className="mt-2 text-sm font-semibold">{value}</dd>
    </div>
  );
}
