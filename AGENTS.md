# Repository Instructions

These instructions apply to every contributor and automated coding agent working in this repository.

## 1. Resume Before You Build

1. Read `README.md`.
2. Read `docs/PROJECT_STATE.md` and identify the exact current phase and next action.
3. Read the relevant phase in `docs/ROADMAP.md`.
4. Inspect the current branch, working tree, recent commits, and existing implementation before editing.
5. Continue from the recorded next action. Do not restart completed work or silently skip a phase.

## 2. Language and Product Boundaries

- English is mandatory for all repository files, source code, code comments, commit messages, user-facing strings, logs, tests, and documentation.
- Build a proprietary custom monitoring product and its own agent, telemetry pipeline, storage model, alerting, and user experience.
- Do not embed or depend on an existing monitoring platform, including Grafana, Prometheus, Zabbix, or Netdata.
- General-purpose development technologies and infrastructure components selected by the project are allowed, including Go, Next.js, PostgreSQL, Redis, gRPC, and Protocol Buffers.
- Do not add dependencies without explaining their purpose, license, operational cost, and alternatives in an ADR when the choice is architectural.

## 3. Incremental Delivery Rules

- Work on the current phase only unless the user explicitly changes priority.
- Prefer small vertical slices that compile, run, and can be tested independently.
- Finish a coherent unit of work before starting another.
- Never leave the default branch knowingly broken.
- Do not replace working code with placeholders merely to make a checklist appear complete.
- Preserve backward compatibility where practical. When a breaking change is necessary, add a migration path and document it.
- Keep interfaces explicit and modules cohesive. Avoid premature microservices and speculative abstractions.
- Keep secrets, credentials, enrollment tokens, and private keys out of source control and logs.

## 4. Code Quality and Comments

- Use clear names, focused functions, explicit error handling, and predictable resource lifecycles.
- Add comments that explain intent, invariants, security boundaries, non-obvious algorithms, and important trade-offs.
- Document exported Go identifiers according to Go conventions.
- Do not comment every trivial line or repeat what the code already says.
- Add tests with the implementation. Tests should cover normal behavior, boundaries, invalid input, and failure paths.
- Use deterministic tests where possible; avoid timing-sensitive assertions without a controlled test clock.

## 5. Git and Commit Discipline

Use Conventional Commits, for example:

- `feat(agent): collect host memory metrics`
- `feat(api): add authenticated agent enrollment`
- `fix(ingestion): bound the retry queue`
- `test(storage): verify duplicate batch handling`
- `docs(roadmap): record phase completion`

Commit each independently reviewable unit. Do not create empty commits. Never claim a change was committed unless the commit exists in GitHub.

## 6. Phase Completion Gate

A phase may be marked `Completed` only when all of the following are true:

1. Every in-scope acceptance criterion is implemented.
2. Relevant automated tests pass.
3. Formatting, linting, and static checks for the affected components pass.
4. Security and failure behavior have been reviewed where relevant.
5. User-facing and developer documentation is updated.
6. The implementation is committed.
7. `docs/PROJECT_STATE.md` and `docs/ROADMAP.md` are updated to identify the next exact action.

If any criterion is not met, keep the phase `In Progress` or `Blocked` and record the reason.

## 7. Reliability and Performance

- Treat telemetry durability, cache behavior, bounded memory, backpressure, retries, idempotency, and shutdown as design requirements.
- Redis is not the durable source of truth for telemetry.
- Do not claim zero crashes, zero lag, perfect accuracy, or million-host capacity without reproducible evidence.
- Define measurement semantics, sampling intervals, clock behavior, and end-to-end latency separately.
- Benchmark representative workloads and record hardware, software versions, workload shape, duration, and results.

## 8. Security

- Use authenticated and encrypted agent communication.
- Use short-lived or revocable enrollment credentials and secure agent identity.
- Verify downloaded installers and binaries before execution.
- Remote actions must use an explicit allowlist, least privilege, authorization checks, audit records, and safe failure behavior.
- Never expose an unrestricted remote shell as a monitoring feature.
- Treat all host telemetry and labels as untrusted input.

## 9. State Updates

At the end of every work session:

1. Record completed tasks and their verification evidence.
2. Record remaining tasks and any blocker.
3. Set one exact next action.
4. Update the current phase and last completed phase.
5. Record relevant commit SHAs.
6. Keep all state documentation in English.

The project is complete only when all roadmap phases required for the agreed release are implemented, tested, documented, and validated against their acceptance criteria.


## 10. README Is the Living Product Guide

The README is a maintained product document, not a one-time placeholder.

- Keep it professional, self-contained, and understandable to an engineer reviewing the repository for the first time.
- Update the README whenever an individual feature reaches Verified status. Describe what the feature does, how it is used, and its actual support/limitations.
- At every phase completion, update the README's current status, feature status table, architecture or workflow details affected by that phase, and next phase.
- Keep planned, specified, in-progress, and verified capabilities clearly distinguished.
- Never list a feature as available merely because its requirement or UI mock has been written.
- Link detailed technical material to the relevant documents so README remains navigable without duplicating every implementation detail.
- The README update is part of the feature/phase completion gate. Do not defer it to a final documentation sprint.
- Before a release, compare every README claim with the feature acceptance matrix and test evidence.
