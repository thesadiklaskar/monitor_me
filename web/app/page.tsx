const capabilities = [
  {
    title: "Host telemetry",
    description: "A native agent will report CPU, memory, disk, network, and host health.",
    status: "Planned",
  },
  {
    title: "Project attribution",
    description: "Workload-aware views will separate supported project resource usage.",
    status: "Planned",
  },
  {
    title: "Operational alerts",
    description: "Rules, recovery events, and audited actions will be introduced in later phases.",
    status: "Planned",
  },
];

export default function HomePage() {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-7xl flex-col px-6 py-8 sm:px-10 lg:px-12">
      <header className="flex items-center justify-between border-b border-[var(--border)] pb-6">
        <div className="flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-xl border border-[var(--border)] bg-[var(--surface-raised)] text-lg font-semibold text-[var(--accent)]">
            M
          </div>
          <div>
            <p className="text-sm font-semibold tracking-wide">MONITOR ME</p>
            <p className="text-xs text-[var(--muted)]">Infrastructure control plane</p>
          </div>
        </div>
        <span className="rounded-full border border-amber-400/30 bg-amber-400/10 px-3 py-1 text-xs font-medium text-amber-200">
          Foundation preview
        </span>
      </header>

      <section className="grid flex-1 content-center gap-10 py-16 lg:grid-cols-[1.2fr_0.8fr] lg:items-center">
        <div className="max-w-3xl">
          <p className="mb-4 text-xs font-semibold uppercase tracking-[0.24em] text-[var(--accent)]">
            Custom infrastructure monitoring
          </p>
          <h1 className="max-w-3xl text-4xl font-semibold leading-tight tracking-tight sm:text-5xl lg:text-6xl">
            Your infrastructure, clearly in view.
          </h1>
          <p className="mt-6 max-w-2xl text-base leading-7 text-[var(--muted)] sm:text-lg">
            Monitor Me is building a simpler way to enroll servers, understand workload
            health, and operate infrastructure from one focused workspace.
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <span className="rounded-lg bg-[var(--accent)] px-4 py-2.5 text-sm font-semibold text-slate-950">
              Platform foundation is running
            </span>
            <span className="text-sm text-[var(--muted)]">
              Server enrollment is not available yet.
            </span>
          </div>
        </div>

        <aside className="rounded-2xl border border-[var(--border)] bg-[var(--surface)] p-6 shadow-2xl shadow-black/20">
          <div className="flex items-center justify-between gap-4">
            <div>
              <h2 className="text-base font-semibold">Workspace overview</h2>
              <p className="mt-1 text-sm text-[var(--muted)]">Initial platform state</p>
            </div>
            <span className="size-2.5 rounded-full bg-amber-300" aria-label="Preview status" />
          </div>

          <div className="mt-6 rounded-xl border border-dashed border-[var(--border)] bg-slate-950/40 px-5 py-8 text-center">
            <div className="mx-auto flex size-11 items-center justify-center rounded-full bg-[var(--surface-raised)] text-xl text-[var(--accent)]">
              +
            </div>
            <h3 className="mt-4 text-sm font-semibold">No hosts enrolled</h3>
            <p className="mx-auto mt-2 max-w-xs text-sm leading-6 text-[var(--muted)]">
              Server inventory will appear here after secure agent enrollment is implemented.
            </p>
          </div>

          <div className="mt-6 grid grid-cols-2 gap-3">
            <div className="rounded-xl bg-[var(--surface-raised)] p-4">
              <p className="text-xs text-[var(--muted)]">Connected servers</p>
              <p className="mt-2 text-2xl font-semibold tabular-nums">—</p>
            </div>
            <div className="rounded-xl bg-[var(--surface-raised)] p-4">
              <p className="text-xs text-[var(--muted)]">Active projects</p>
              <p className="mt-2 text-2xl font-semibold tabular-nums">—</p>
            </div>
          </div>
        </aside>
      </section>

      <section className="border-t border-[var(--border)] py-8">
        <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h2 className="text-sm font-semibold">Platform capabilities</h2>
            <p className="mt-1 text-xs text-[var(--muted)]">
              Planned capabilities are not presented as live monitoring data.
            </p>
          </div>
          <span className="text-xs text-[var(--muted)]">Delivery phase 2 of 5</span>
        </div>
        <div className="grid gap-3 md:grid-cols-3">
          {capabilities.map((capability) => (
            <article
              key={capability.title}
              className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-5"
            >
              <div className="flex items-center justify-between gap-3">
                <h3 className="text-sm font-semibold">{capability.title}</h3>
                <span className="rounded-full bg-slate-700/60 px-2.5 py-1 text-[10px] font-medium text-slate-300">
                  {capability.status}
                </span>
              </div>
              <p className="mt-3 text-sm leading-6 text-[var(--muted)]">
                {capability.description}
              </p>
            </article>
          ))}
        </div>
      </section>

      <footer className="flex flex-wrap items-center justify-between gap-2 border-t border-[var(--border)] pt-5 text-xs text-[var(--muted)]">
        <span>Monitor Me · Custom monitoring platform</span>
        <span>Development preview · No production telemetry connected</span>
      </footer>
    </main>
  );
}
