import { ArrowUpRight, Clock3, Code2, MessageCircle, Star } from "lucide-react";
import { Link } from "react-router";

import { Alert, AlertDescription } from "../../../components/ui/alert";
import { Badge } from "../../../components/ui/badge";
import { Button } from "../../../components/ui/button";
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
} from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import type { IssueSearchItem } from "../../../shared/api/generated";
import { appRoutes } from "../../../shared/config/app-config";
import {
  formatCompactNumber,
  formatDate,
  formatPercentage,
} from "../../../shared/lib/format";
import { cn } from "../../../shared/lib/cn";
import {
  scorePresentation,
  skillPresentation,
  warningPresentation,
} from "../model/search-presentation";

type RecommendationCardProps = {
  item: IssueSearchItem;
  rank: number;
};

export function RecommendationCard({ item, rank }: RecommendationCardProps) {
  const score = scorePresentation(item.recommendation.score);
  const detailPath = appRoutes.issue(
    item.repository.owner,
    item.repository.name,
    item.issue.number,
  );
  return (
    <Card aria-labelledby={`issue-result-${rank}`} className="overflow-hidden">
      <CardHeader className="border-b border-border bg-muted/35">
        <div className="flex flex-wrap items-start justify-between gap-5">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className="font-mono">#{rank}</span>
              <span>{item.repository.fullName}</span>
              <span aria-hidden="true">·</span>
              <span className="inline-flex items-center gap-1">
                <Icon className="size-3.5" icon={Star} />
                {formatCompactNumber(item.repository.stars)}
              </span>
              <span aria-hidden="true">·</span>
              <span>Updated {formatDate(item.issue.updatedAt)}</span>
            </div>
            <h2
              className="mt-3 text-xl leading-7 font-semibold tracking-[-0.03em]"
              id={`issue-result-${rank}`}
            >
              {item.issue.title}
            </h2>
          </div>
          <div
            aria-label={`${item.recommendation.score} out of 100, ${score.label}`}
            aria-valuemax={100}
            aria-valuemin={0}
            aria-valuenow={item.recommendation.score}
            className={cn(
              "grid size-20 shrink-0 place-items-center rounded-2xl border text-center",
              score.className,
            )}
            role="meter"
          >
            <span>
              <strong className="block font-mono text-2xl leading-none">
                {item.recommendation.score}
              </strong>
              <span className="mt-1 block text-[0.65rem] font-semibold uppercase">
                {score.label}
              </span>
            </span>
          </div>
        </div>
        <div className="mt-3 flex flex-wrap gap-2">
          {item.repository.mainLanguage ? (
            <Badge variant="info">
              <Icon className="size-3.5" icon={Code2} />
              {item.repository.mainLanguage}
            </Badge>
          ) : null}
          <Badge variant="neutral">
            Difficulty {item.difficulty.level}: {item.difficulty.label}
          </Badge>
          <Badge variant="neutral">
            <Icon className="size-3.5" icon={Clock3} />
            {item.effort.label}
          </Badge>
          <Badge variant="neutral">
            <Icon className="size-3.5" icon={MessageCircle} />
            {formatCompactNumber(item.issue.comments)} comments
          </Badge>
          {item.repository.isArchived ? (
            <Badge variant="warning">Archived repository</Badge>
          ) : null}
        </div>
      </CardHeader>

      <CardContent className="grid gap-5 p-5 sm:p-6">
        <section aria-label="Issue labels">
          <p className="text-xs font-semibold tracking-[0.12em] text-muted-foreground uppercase">
            Labels
          </p>
          <div className="mt-2 flex flex-wrap gap-2">
            {item.issue.labels.length > 0 ? (
              item.issue.labels.map((label) => (
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
        </section>

        <section
          aria-labelledby={`skill-match-${rank}`}
          className="rounded-xl border border-border bg-muted/35 p-4"
        >
          <div className="flex flex-wrap items-end justify-between gap-3">
            <div>
              <h3 className="font-semibold" id={`skill-match-${rank}`}>
                Skill match
              </h3>
              <p className="mt-1 text-xs text-muted-foreground">
                {item.recommendation.skillMatch.matched} of{" "}
                {item.recommendation.skillMatch.denominator} comparable skills
              </p>
            </div>
            <strong className="font-mono text-xl text-accent">
              {formatPercentage(item.recommendation.skillMatch.percentage)}
            </strong>
          </div>
          <div className="mt-3 flex flex-wrap gap-2">
            {item.recommendation.skillMatch.skills.length > 0 ? (
              item.recommendation.skillMatch.skills.map((skill) => (
                <Badge
                  key={`${skill.technology}-${skill.status}`}
                  variant={skillPresentation(skill.status)}
                >
                  {skill.technology}: {skill.status}
                </Badge>
              ))
            ) : (
              <span className="text-sm text-muted-foreground">
                No comparable skill evidence
              </span>
            )}
          </div>
        </section>

        <section aria-labelledby={`reasons-${rank}`}>
          <h3 className="font-semibold" id={`reasons-${rank}`}>
            Why it ranks here
          </h3>
          <ul className="mt-2 grid gap-2 text-sm leading-6 text-muted-foreground">
            {item.recommendation.reasons.map((reason) => (
              <li className="flex gap-2" key={reason}>
                <span aria-hidden="true" className="text-accent">
                  •
                </span>
                <span>{reason}</span>
              </li>
            ))}
          </ul>
        </section>

        {item.recommendation.warnings.length > 0 ? (
          <section aria-label="Recommendation warnings" className="grid gap-2">
            {item.recommendation.warnings.map((warning) => (
              <Alert
                key={`${warning.code}-${warning.message}`}
                variant={warningPresentation(warning.severity)}
              >
                <AlertDescription>{warning.message}</AlertDescription>
              </Alert>
            ))}
          </section>
        ) : null}
      </CardContent>

      <CardFooter className="flex-wrap border-t border-border bg-muted/20 p-5 sm:p-6">
        <Button asChild>
          <Link to={detailPath}>View recommendation details</Link>
        </Button>
        <Button asChild variant="outline">
          <a href={item.issue.url} rel="noreferrer" target="_blank">
            Open GitHub issue
            <Icon icon={ArrowUpRight} />
          </a>
        </Button>
      </CardFooter>
    </Card>
  );
}
