import {
  Activity,
  ArrowUpRight,
  Braces,
  CircleHelp,
  FileCheck2,
  Gauge,
  GitPullRequestArrow,
  Info,
  LockKeyhole,
  Radar,
  Sparkles,
} from "lucide-react";

import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "../components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "../components/ui/dialog";
import { Icon } from "../components/ui/icon";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "../components/ui/popover";
import { ProfileSearchForm } from "../features/profile/components/ProfileSearchForm";

const recommendationSignals = [
  {
    detail: "Languages, frameworks, and manifest evidence",
    icon: Braces,
    label: "Technology fit",
    score: "30",
  },
  {
    detail: "Description clarity, scope, and verification",
    icon: FileCheck2,
    label: "Issue quality",
    score: "20",
  },
  {
    detail: "Docs, tests, CI, activity, and responsiveness",
    icon: Activity,
    label: "Project signals",
    score: "40",
  },
  {
    detail: "Assignments and conservative claim detection",
    icon: GitPullRequestArrow,
    label: "Availability",
    score: "10",
  },
] as const;

const journeySteps = [
  {
    description:
      "We inspect up to 20 public, non-fork repositories—never private data.",
    icon: Radar,
    number: "01",
    title: "Read the public signal",
  },
  {
    description:
      "Languages and frameworks are normalized from repository and manifest evidence.",
    icon: Braces,
    number: "02",
    title: "Build your technology map",
  },
  {
    description:
      "Every future recommendation shows its score, evidence, and uncertainty.",
    icon: Gauge,
    number: "03",
    title: "Explain the match",
  },
] as const;

export function HomePage() {
  return (
    <>
      <section className="mx-auto grid w-full max-w-7xl gap-12 px-5 py-16 sm:px-8 sm:py-24 lg:grid-cols-[minmax(0,1.08fr)_minmax(22rem,0.72fr)] lg:items-center lg:gap-18 lg:px-10 lg:py-28">
        <div>
          <Badge variant="accent">
            <Icon icon={Sparkles} />
            Open source, matched to your real work
          </Badge>
          <h1 className="mt-7 max-w-4xl text-5xl leading-[0.98] font-semibold tracking-[-0.065em] text-balance sm:text-6xl lg:text-[5.25rem]">
            Your next contribution,{" "}
            <span className="text-accent">decoded.</span>
          </h1>
          <p className="mt-7 max-w-2xl text-lg leading-8 text-muted-foreground sm:text-xl">
            Turn a public GitHub profile into an evidence-based technology map,
            then use it to find issues you can realistically finish.
          </p>

          <Card
            className="mt-10 border-accent/20 bg-surface/94 p-2 shadow-[0_28px_80px_-48px_var(--accent-glow)]"
            id="analyze"
          >
            <CardContent className="p-5 sm:p-6">
              <ProfileSearchForm />
              <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2 border-t border-border pt-4 text-xs text-muted-foreground">
                <span className="inline-flex items-center gap-1.5">
                  <Icon className="size-3.5" icon={LockKeyhole} />
                  No sign-in
                </span>
                <span>Public data only</span>
                <Popover>
                  <PopoverTrigger asChild>
                    <button
                      className="inline-flex items-center gap-1.5 rounded-md font-medium text-foreground outline-none hover:text-accent focus-visible:ring-2 focus-visible:ring-ring"
                      type="button"
                    >
                      <Icon className="size-3.5" icon={Info} />
                      What gets analyzed?
                    </button>
                  </PopoverTrigger>
                  <PopoverContent align="start">
                    IssueScout reads the same public profile and repository
                    metadata visible to anyone on GitHub. This anonymous flow
                    never opens a database connection.
                  </PopoverContent>
                </Popover>
              </div>
            </CardContent>
          </Card>
        </div>

        <div className="relative mx-auto w-full max-w-xl">
          <div
            aria-hidden="true"
            className="absolute -inset-8 -z-10 rounded-full bg-accent/8 blur-3xl"
          />
          <Card className="overflow-hidden">
            <CardHeader className="border-b border-border bg-muted/60">
              <div className="flex items-center justify-between gap-4">
                <div>
                  <p className="font-mono text-xs tracking-[0.16em] text-accent uppercase">
                    Explainable model
                  </p>
                  <CardTitle className="mt-2">Recommendation anatomy</CardTitle>
                </div>
                <div className="grid size-18 place-items-center rounded-full border-8 border-accent-soft bg-surface font-mono text-xl font-bold text-accent">
                  100
                  <span className="sr-only">points total</span>
                </div>
              </div>
            </CardHeader>
            <CardContent className="grid gap-2 p-3 sm:p-4">
              {recommendationSignals.map((signal) => (
                <div
                  className="grid grid-cols-[2.5rem_minmax(0,1fr)_2.5rem] items-center gap-3 rounded-xl border border-transparent p-3 transition-colors hover:border-border hover:bg-muted/70"
                  key={signal.label}
                >
                  <span className="grid size-10 place-items-center rounded-lg bg-accent-soft text-accent-soft-foreground">
                    <Icon icon={signal.icon} />
                  </span>
                  <div>
                    <p className="text-sm font-semibold">{signal.label}</p>
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">
                      {signal.detail}
                    </p>
                  </div>
                  <span className="font-mono text-sm font-bold text-accent">
                    {signal.score}
                  </span>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </section>

      <section
        className="border-y border-border bg-surface/48"
        id="how-it-works"
      >
        <div className="mx-auto w-full max-w-7xl px-5 py-18 sm:px-8 sm:py-24 lg:px-10">
          <div className="flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p className="font-mono text-xs tracking-[0.18em] text-accent uppercase">
                From profile to proof
              </p>
              <h2 className="mt-4 max-w-2xl text-3xl font-semibold tracking-[-0.045em] text-balance sm:text-4xl">
                A clear path from what you built to what you can build next.
              </h2>
            </div>
            <Dialog>
              <DialogTrigger asChild>
                <Button variant="outline">
                  <Icon icon={CircleHelp} />
                  See the evidence model
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Evidence before confidence</DialogTitle>
                  <DialogDescription>
                    IssueScout separates facts, bounded samples, and heuristics
                    so a recommendation never looks more certain than its data.
                  </DialogDescription>
                </DialogHeader>
                <div className="grid gap-3">
                  {[
                    "Repository language is medium-confidence evidence.",
                    "A matching manifest dependency is high-confidence evidence.",
                    "Missing or partial data stays explicitly unknown.",
                    "Every score component has a fixed maximum.",
                  ].map((item) => (
                    <div
                      className="flex gap-3 rounded-xl bg-muted p-4 text-sm leading-6"
                      key={item}
                    >
                      <Icon className="mt-1 text-accent" icon={ArrowUpRight} />
                      <p>{item}</p>
                    </div>
                  ))}
                </div>
              </DialogContent>
            </Dialog>
          </div>

          <div className="mt-12 grid gap-4 lg:grid-cols-3">
            {journeySteps.map((step) => (
              <Card
                className="group relative overflow-hidden"
                key={step.number}
              >
                <CardHeader className="relative">
                  <span className="font-mono text-xs font-semibold tracking-[0.16em] text-muted-foreground">
                    {step.number}
                  </span>
                  <span className="mt-8 grid size-12 place-items-center rounded-xl bg-accent-soft text-accent-soft-foreground transition-transform group-hover:-translate-y-1">
                    <Icon className="size-5" icon={step.icon} />
                  </span>
                  <CardTitle className="mt-3">{step.title}</CardTitle>
                  <CardDescription>{step.description}</CardDescription>
                </CardHeader>
              </Card>
            ))}
          </div>
        </div>
      </section>

      <section className="mx-auto w-full max-w-7xl px-5 py-18 sm:px-8 sm:py-24 lg:px-10">
        <Card className="grid overflow-hidden border-accent/20 lg:grid-cols-[1fr_auto] lg:items-center">
          <CardHeader className="p-7 sm:p-10">
            <Badge className="w-fit" variant="success">
              Anonymous by design
            </Badge>
            <CardTitle className="mt-3 text-2xl sm:text-3xl">
              Start with a username. Keep control of the journey.
            </CardTitle>
            <CardDescription className="max-w-2xl text-base">
              Profile analysis is read-only, public, and database-free. You can
              retry, change profiles, or leave without leaving account data
              behind.
            </CardDescription>
          </CardHeader>
          <div className="p-7 pt-0 sm:p-10 sm:pt-0 lg:pt-10">
            <Button asChild size="large">
              <a href="#analyze">
                Analyze your profile
                <Icon icon={ArrowUpRight} />
              </a>
            </Button>
          </div>
        </Card>
      </section>
    </>
  );
}
