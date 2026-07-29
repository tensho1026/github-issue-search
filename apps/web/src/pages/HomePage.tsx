const productSignals = [
  {
    label: "Skill fit",
    value: "Language and framework matching",
  },
  {
    label: "Effort",
    value: "Rule-based difficulty and time estimates",
  },
  {
    label: "Repository health",
    value: "Activity, documentation, tests, and CI signals",
  },
] as const;

export function HomePage() {
  return (
    <main className="min-h-screen overflow-hidden bg-slate-950 text-slate-100">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 top-0 -z-0 h-[34rem] bg-[radial-gradient(circle_at_top_left,rgba(45,212,191,0.16),transparent_42%),radial-gradient(circle_at_80%_15%,rgba(56,189,248,0.14),transparent_36%)]"
      />

      <div className="relative mx-auto flex min-h-screen w-full max-w-6xl flex-col px-6 py-8 sm:px-10 lg:px-12">
        <header className="flex items-center justify-between border-b border-white/10 pb-6">
          <a
            className="inline-flex items-center gap-3 rounded-md text-sm font-semibold tracking-[0.18em] text-white uppercase focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-teal-300"
            href="/"
          >
            <span
              aria-hidden="true"
              className="grid size-9 place-items-center rounded-xl border border-teal-300/30 bg-teal-300/10 text-teal-200"
            >
              IS
            </span>
            IssueScout
          </a>
          <span className="rounded-full border border-white/10 px-3 py-1 text-xs text-slate-400">
            MVP foundation
          </span>
        </header>

        <section className="grid flex-1 items-center gap-16 py-20 lg:grid-cols-[1.2fr_0.8fr]">
          <div>
            <p className="mb-5 text-sm font-semibold tracking-[0.22em] text-teal-300 uppercase">
              Open source, matched to you
            </p>
            <h1 className="max-w-3xl text-5xl leading-[0.98] font-semibold tracking-[-0.05em] text-balance sm:text-6xl lg:text-7xl">
              Find the issue you can finish.
            </h1>
            <p className="mt-8 max-w-2xl text-lg leading-8 text-slate-300">
              IssueScout evaluates GitHub issues against your real technology
              profile, available time, and the health of the project behind
              them.
            </p>
            <div className="mt-10 flex flex-wrap items-center gap-4">
              <span className="rounded-xl bg-teal-300 px-5 py-3 text-sm font-semibold text-slate-950">
                Profile analysis coming next
              </span>
              <span className="text-sm text-slate-400">
                No account or database required for the MVP
              </span>
            </div>
          </div>

          <aside
            aria-label="IssueScout recommendation signals"
            className="rounded-3xl border border-white/10 bg-white/[0.04] p-4 shadow-2xl shadow-slate-950/40 backdrop-blur"
          >
            <div className="rounded-2xl border border-white/10 bg-slate-900/80 p-6">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium text-slate-300">
                  Recommendation model
                </p>
                <span className="rounded-full bg-sky-300/10 px-2.5 py-1 text-xs text-sky-200">
                  Explainable
                </span>
              </div>
              <div className="mt-8 space-y-3">
                {productSignals.map((signal, index) => (
                  <div
                    className="grid grid-cols-[2rem_1fr] gap-4 rounded-xl border border-white/[0.07] bg-white/[0.03] p-4"
                    key={signal.label}
                  >
                    <span className="font-mono text-sm text-teal-300">
                      0{index + 1}
                    </span>
                    <div>
                      <p className="font-medium text-white">{signal.label}</p>
                      <p className="mt-1 text-sm leading-6 text-slate-400">
                        {signal.value}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </aside>
        </section>
      </div>
    </main>
  );
}
