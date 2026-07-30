import {
  ArrowLeft,
  ArrowUpRight,
  BookOpen,
  Box,
  Code2,
  Eye,
  GitFork,
  Star,
  Users,
} from "lucide-react";
import { useMemo, useState } from "react";
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
  CardDescription,
  CardHeader,
  CardTitle,
} from "../../../components/ui/card";
import { Icon } from "../../../components/ui/icon";
import {
  createDefaultSearchFilters,
  encodeSearchParams,
} from "../../issue-search/model/search-filters";
import {
  createDefaultRepositoryFilters,
  encodeRepositorySearchParams,
} from "../../repository-discovery/model/repository-filters";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../../../components/ui/select";
import { appRoutes, externalLinks } from "../../../shared/config/app-config";
import {
  formatCompactNumber,
  formatDate,
  formatPercentage,
} from "../../../shared/lib/format";
import type { ProfileSnapshot } from "../api/useProfileSnapshot";
import {
  featuredRepositories,
  profileTechnologyTags,
  sortLanguages,
  type LanguageOrder,
} from "../model/profile-view";
import { ProfileSearchForm } from "./ProfileSearchForm";
import { ProfileExtendedAnalytics } from "./ProfileExtendedAnalytics";

type ProfileDashboardProps = {
  snapshot: ProfileSnapshot;
};

function Metric({ label, value }: { label: string; value: number }) {
  return (
    <div className="rounded-xl bg-muted p-3 text-center">
      <p className="text-lg font-semibold tracking-[-0.03em]">
        {formatCompactNumber(value)}
      </p>
      <p className="mt-1 text-xs text-muted-foreground">{label}</p>
    </div>
  );
}

export function ProfileDashboard({ snapshot }: ProfileDashboardProps) {
  const { analysis, analysisMeta, user, userMeta } = snapshot;
  const [languageOrder, setLanguageOrder] = useState<LanguageOrder>("usage");
  const languages = useMemo(
    () => sortLanguages(analysis.languages, languageOrder),
    [analysis.languages, languageOrder],
  );
  const technologies = useMemo(
    () => profileTechnologyTags(analysis),
    [analysis],
  );
  const repositories = useMemo(() => featuredRepositories(user), [user]);
  const issueSearchTarget = useMemo(() => {
    const filters = createDefaultSearchFilters(user.login);
    filters.languages = analysis.languages
      .map((language) => language.name)
      .slice(0, 10);
    filters.frameworks = analysis.frameworks.slice(0, 10);
    return {
      pathname: appRoutes.search,
      search: encodeSearchParams(filters, false).toString(),
    };
  }, [analysis.frameworks, analysis.languages, user.login]);
  const repositoryDiscoveryTarget = useMemo(() => {
    const filters = createDefaultRepositoryFilters();
    filters.languages = analysis.languages
      .map((language) => language.name)
      .slice(0, 10);
    filters.technologies = analysis.frameworks.slice(0, 10);
    return {
      pathname: appRoutes.repositories,
      search: encodeRepositorySearchParams(filters, false).toString(),
    };
  }, [analysis.frameworks, analysis.languages]);
  const rateLimitRemaining = [
    userMeta.rateLimitRemaining,
    analysisMeta.rateLimitRemaining,
  ].reduce<number | undefined>((lowest, remaining) => {
    if (remaining === undefined) {
      return lowest;
    }
    return lowest === undefined ? remaining : Math.min(lowest, remaining);
  }, undefined);

  return (
    <div className="mx-auto w-full max-w-7xl px-5 py-10 sm:px-8 sm:py-14 lg:px-10">
      <Link
        className="inline-flex items-center gap-2 rounded-lg text-sm font-medium text-muted-foreground outline-none transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
        to={appRoutes.home}
      >
        <Icon icon={ArrowLeft} />
        Analyze another profile
      </Link>

      <section
        aria-labelledby="profile-heading"
        className="mt-7 grid gap-5 lg:grid-cols-[0.78fr_1.22fr]"
      >
        <Card className="overflow-hidden">
          <CardHeader className="border-b border-border bg-muted/50">
            <div className="flex items-start gap-4">
              <img
                alt={`${user.login} GitHub avatar`}
                className="size-20 rounded-2xl border border-border bg-muted object-cover shadow-sm"
                height={80}
                referrerPolicy="no-referrer"
                src={user.avatarUrl}
                width={80}
              />
              <div className="min-w-0 flex-1">
                <Badge variant="accent">Public profile</Badge>
                <h1
                  className="mt-3 truncate text-2xl font-semibold tracking-[-0.04em]"
                  id="profile-heading"
                >
                  {user.name || user.login}
                </h1>
                <a
                  className="mt-1 inline-flex items-center gap-1 rounded-md text-sm text-muted-foreground outline-none hover:text-accent focus-visible:ring-2 focus-visible:ring-ring"
                  href={externalLinks.gitHubProfile(user.login)}
                  rel="noreferrer"
                  target="_blank"
                >
                  @{user.login}
                  <Icon className="size-3.5" icon={ArrowUpRight} />
                </a>
              </div>
            </div>
            <CardDescription className="mt-3">
              {user.bio || "No public bio is available for this profile."}
            </CardDescription>
          </CardHeader>
          <CardContent className="grid grid-cols-3 gap-2 p-4 sm:p-5">
            <Metric label="Repositories" value={user.publicRepos} />
            <Metric label="Followers" value={user.followers} />
            <Metric label="Following" value={user.following} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-start justify-between gap-4">
              <div>
                <p className="font-mono text-xs tracking-[0.16em] text-accent uppercase">
                  Technology fingerprint
                </p>
                <CardTitle className="mt-2 text-2xl">
                  Built from public repository evidence
                </CardTitle>
              </div>
              <Badge variant="neutral">
                {analysis.repositoriesAnalyzed} repositories analyzed
              </Badge>
            </div>
            <CardDescription>
              Languages and frameworks are normalized from a bounded set of
              public, non-fork repositories.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {technologies.length > 0 ? (
              <ul
                aria-label="Detected technologies"
                className="flex flex-wrap gap-2"
              >
                {technologies.map((technology) => (
                  <li key={technology}>
                    <Badge variant="accent">
                      <Icon className="size-3.5" icon={Code2} />
                      {technology}
                    </Badge>
                  </li>
                ))}
              </ul>
            ) : (
              <div className="rounded-xl border border-dashed border-border bg-muted/50 p-5">
                <p className="font-semibold">No technology signal yet</p>
                <p className="mt-2 text-sm leading-6 text-muted-foreground">
                  This profile has no eligible public repository language or
                  framework evidence in the bounded window.
                </p>
              </div>
            )}
            {rateLimitRemaining !== undefined ? (
              <p className="mt-5 text-xs text-muted-foreground">
                GitHub API requests remaining:{" "}
                <strong className="text-foreground">
                  {formatCompactNumber(rateLimitRemaining)}
                </strong>
              </p>
            ) : null}
          </CardContent>
        </Card>
      </section>

      {analysis.warnings.length > 0 ? (
        <div className="mt-5 grid gap-3" aria-label="Analysis warnings">
          {analysis.warnings.map((warning, index) => (
            <Alert
              key={`${warning.code}-${warning.repository ?? index}`}
              variant="warning"
            >
              <AlertTitle>Some evidence was unavailable</AlertTitle>
              <AlertDescription>
                {warning.message}
                {warning.repository ? ` (${warning.repository})` : ""}
              </AlertDescription>
            </Alert>
          ))}
        </div>
      ) : null}

      <Card className="mt-5 overflow-hidden border-accent/20">
        <CardHeader className="sm:flex sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="font-mono text-xs tracking-[0.16em] text-accent uppercase">
              Next step
            </p>
            <CardTitle className="mt-2">
              Turn this profile into a bounded issue search
            </CardTitle>
            <CardDescription>
              Detected languages and frameworks are prefilled. Review every
              filter before GitHub search begins.
            </CardDescription>
          </div>
          <div className="mt-3 flex flex-wrap gap-3 sm:mt-0">
            <Button asChild className="shrink-0">
              <Link to={issueSearchTarget}>
                Find matching issues
                <Icon icon={ArrowUpRight} />
              </Link>
            </Button>
            <Button asChild className="shrink-0" variant="outline">
              <Link to={repositoryDiscoveryTarget}>
                Discover repositories
                <Icon icon={ArrowUpRight} />
              </Link>
            </Button>
          </div>
        </CardHeader>
      </Card>

      <section
        aria-labelledby="languages-heading"
        className="mt-5 grid gap-5 lg:grid-cols-[1.15fr_0.85fr]"
      >
        <Card>
          <CardHeader className="flex-row items-start justify-between gap-4">
            <div>
              <CardTitle id="languages-heading">
                Language distribution
              </CardTitle>
              <CardDescription>
                Share of the bounded repository sample.
              </CardDescription>
            </div>
            <div>
              <label className="sr-only" htmlFor="language-order">
                Sort languages
              </label>
              <Select
                onValueChange={(value) =>
                  setLanguageOrder(value as LanguageOrder)
                }
                value={languageOrder}
              >
                <SelectTrigger aria-label="Sort languages" id="language-order">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="usage">Most used</SelectItem>
                  <SelectItem value="alphabetical">A–Z</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </CardHeader>
          <CardContent>
            {languages.length > 0 ? (
              <ul className="grid gap-5">
                {languages.map((language) => (
                  <li className="grid gap-2" key={language.name}>
                    <div className="flex items-center justify-between gap-4 text-sm">
                      <span className="font-medium">{language.name}</span>
                      <span className="font-mono text-muted-foreground">
                        {formatPercentage(language.percentage)}
                      </span>
                    </div>
                    <div
                      aria-label={`${language.name} ${formatPercentage(language.percentage)}`}
                      aria-valuemax={100}
                      aria-valuemin={0}
                      aria-valuenow={language.percentage}
                      className="h-2.5 overflow-hidden rounded-full bg-muted-strong"
                      role="progressbar"
                    >
                      <div
                        className="h-full rounded-full bg-accent"
                        style={{
                          width: formatPercentage(language.percentage),
                        }}
                      />
                    </div>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="rounded-xl bg-muted p-5 text-sm leading-6 text-muted-foreground">
                No primary language percentages were available in the analyzed
                repository window.
              </p>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Framework evidence</CardTitle>
            <CardDescription>
              Detected from bounded manifest files, never guessed from trends.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {analysis.frameworks.length > 0 ? (
              <ul className="grid gap-2">
                {analysis.frameworks.map((framework) => (
                  <li
                    className="flex items-center gap-3 rounded-xl border border-border bg-muted/45 p-3 text-sm font-medium"
                    key={framework}
                  >
                    <span className="grid size-9 place-items-center rounded-lg bg-accent-soft text-accent-soft-foreground">
                      <Icon icon={Box} />
                    </span>
                    {framework}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="rounded-xl bg-muted p-5 text-sm leading-6 text-muted-foreground">
                No supported framework dependency was detected. This is an
                explicit empty result, not a negative skill judgment.
              </p>
            )}
          </CardContent>
        </Card>
      </section>

      <ProfileExtendedAnalytics analysis={analysis} />

      <section aria-labelledby="repositories-heading" className="mt-5">
        <Card>
          <CardHeader>
            <CardTitle id="repositories-heading">Repository evidence</CardTitle>
            <CardDescription>
              Featured by public star count for display only. Recommendation
              scoring remains on the API.
            </CardDescription>
          </CardHeader>
          <CardContent>
            {repositories.length > 0 ? (
              <ul className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {repositories.map((repository) => (
                  <li key={repository.fullName}>
                    <a
                      className="group flex h-full flex-col rounded-xl border border-border bg-muted/35 p-4 outline-none transition-[border-color,background-color,transform] hover:-translate-y-0.5 hover:border-accent/35 hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
                      href={repository.url}
                      rel="noreferrer"
                      target="_blank"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <span className="grid size-9 place-items-center rounded-lg bg-surface text-accent">
                          <Icon icon={BookOpen} />
                        </span>
                        {repository.isArchived ? (
                          <Badge variant="warning">Archived</Badge>
                        ) : null}
                      </div>
                      <p className="mt-4 truncate font-semibold">
                        {repository.name}
                      </p>
                      <p className="mt-2 line-clamp-2 flex-1 text-sm leading-6 text-muted-foreground">
                        {repository.description || "No public description."}
                      </p>
                      <div className="mt-4 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                        {repository.mainLanguage ? (
                          <span>{repository.mainLanguage}</span>
                        ) : null}
                        <span className="inline-flex items-center gap-1">
                          <Icon className="size-3.5" icon={Star} />
                          {formatCompactNumber(repository.stars)}
                        </span>
                        <span className="inline-flex items-center gap-1">
                          <Icon className="size-3.5" icon={GitFork} />
                          {formatCompactNumber(repository.forks)}
                        </span>
                      </div>
                      <p className="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
                        <Icon className="size-3.5" icon={Eye} />
                        Updated {formatDate(repository.updatedAt)}
                      </p>
                    </a>
                  </li>
                ))}
              </ul>
            ) : (
              <div className="rounded-xl border border-dashed border-border bg-muted/40 p-7 text-center">
                <Icon
                  className="mx-auto size-6 text-muted-foreground"
                  icon={Users}
                />
                <p className="mt-3 font-semibold">
                  No eligible public repositories
                </p>
                <p className="mx-auto mt-2 max-w-lg text-sm leading-6 text-muted-foreground">
                  The account exists, but the bounded profile window contains no
                  public, non-fork repositories to analyze.
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      </section>

      <Card className="mt-5 border-accent/20">
        <CardHeader>
          <CardTitle>Analyze a different public profile</CardTitle>
          <CardDescription>
            Server state stays in the query cache; this form controls only the
            next route.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ProfileSearchForm compact />
        </CardContent>
      </Card>
    </div>
  );
}
