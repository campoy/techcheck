# Milestones

The single source of truth for delivery milestones of the
[design doc](design_doc.md): the components each milestone delivers and
their status. When a milestone is done, its entry
links to the git commit that completed it (use the merge/final commit).

## Process

Each milestone is delivered test-first, in two phases:

1. **Tests first.** Write the unit and integration tests that validate the
   behaviors we want to observe once the milestone is complete — before any
   implementation. These land as the first commit of the milestone's PR and
   require explicit review and approval (as PR review) before implementation
   starts. At this point the tests fail; that is the expected state, and the
   PR's checks are red.
2. **Implementation.** Build the milestone's components on the same PR,
   making the relevant tests pass as each component lands. A component is
   not done until the tests covering it pass. The milestone merges as one
   PR once all checks are green — required status checks mean a red
   tests-only PR cannot merge on its own.

A milestone is **done** when every test from its first commit passes
unmodified. If implementation reveals that a test encoded the wrong
expectation, changing that test is its own commit with a justification —
never silently adjusted to match the implementation. The completing commit
is then linked from the table below.

The behaviors each milestone's tests encode are its functional requirements:
every milestone section lists the FR IDs (from
[functional_requirements.md](functional_requirements.md)) it delivers, and
those IDs are the checklist for that milestone's first test commit. A
requirement marked *partial* is finished by a later milestone, which lists
it again. FR-8.4 (manual retention) requires no implementation and is
assigned to no milestone.

| Milestone | Summary | Delivers | Status |
|---|---|---|---|
| [M0](#m0-design) | Requirements and design docs | — | ✅ Done ([`c3128a6`](https://github.com/campoy/techcheck/commit/c3128a6)) |
| [M1](#m1-skeleton) | Compose stack, worker and api binaries, trivial workflow | FR-1.4 | ✅ Done ([#3](https://github.com/campoy/techcheck/pull/3)) |
| [M2](#m2-linear-research) | Linear research pipeline, brief to disk | FR-2, FR-3, FR-6 core | 🚧 In progress |
| [M3](#m3-corpus) | Corpus ingestion and retrieval | FR-4, FR-8.3 | ⬜ Not started |
| [M4](#m4-loop--review) | Analysis loop, human review, accumulation | FR-5, FR-7, FR-1.3, FR-8.1 | ⬜ Not started |
| [M5](#m5-cost--polish) | Token budgets, cost reporting, hardening | FR-9.2, FR-9.3 | ⬜ Not started |

## M0: Design

**Status: ✅ Done** — commits
[`c1276a6`](https://github.com/campoy/techcheck/commit/c1276a6) (license),
[`fec66db`](https://github.com/campoy/techcheck/commit/fec66db) (functional
requirements),
[`db57ae7`](https://github.com/campoy/techcheck/commit/db57ae7) (design doc),
[`77da097`](https://github.com/campoy/techcheck/commit/77da097) (README),
[`a55ae5c`](https://github.com/campoy/techcheck/commit/a55ae5c) (milestone
tracker), and
[`c3128a6`](https://github.com/campoy/techcheck/commit/c3128a6) (FR mapping,
completing commit).

Components:

- Functional requirements (FR-1–FR-9), technology-agnostic.
- Design doc for the Temporal/Go implementation.
- README and MIT license.
- This milestone tracker, with the test-first process and the
  milestone-to-requirement mapping.

## M1: Skeleton

**Status: ✅ Done** — tests in
[#2](https://github.com/campoy/techcheck/pull/2), implementation in
[#3](https://github.com/campoy/techcheck/pull/3).

**Delivers:** FR-1.4 (the API exists and reaches Temporal); FR-9.1 partial
(event history is captured and inspectable, though there is little to
inspect yet). M1 is otherwise pure infrastructure, so its first-commit tests
are smoke tests — the stack comes up, a workflow round-trips — rather than
FR tests.

Components:

- Docker Compose file: Temporal server (Postgres persistence), Postgres 16
  with pgvector, Temporal Web UI.
- `worker/` binary registering a trivial workflow and activity.
- `api/` binary: stdlib `net/http` server with `/healthz` and one endpoint
  that starts the trivial workflow.
- One end-to-end request inspectable in the Temporal Web UI.

## M2: Linear research

**Status: 🚧 In progress** — tests under review.

**Delivers:** FR-1.1, FR-1.2, FR-2.1, FR-2.2, FR-3.1, FR-3.2, FR-3.4,
FR-6.1–FR-6.3, FR-8.2, FR-9.4. Partial: FR-5.4 (fixed search budget; the
real per-iteration cap arrives with the loop in M4) and FR-6.1's
`comparable_precedents` field (empty until M3). FR-6.4 (brief quality
comparable to manual evaluations) gets its first check here — re-run a
previously evaluated company and compare against the hand-written brief —
but is only finalized in M4; it is validated by manual review, not by an
automated test.

Components:

- `CompanyResearch` workflow, linear version (no loop, no corpus):
  `PlanResearch` → `WebResearch` → `ExtractFindings` → `GenerateBrief`
  with a fixed search budget.
- LLM client (hosted API via open-source Go SDK) with the two-tier
  model interface (extract vs. reason).
- `Searcher` interface with a paid search API implementation; `FetchPage`
  with readability extraction.
- Schema-constrained structured output into `Finding` and `CompanyBrief`
  structs; findings rejected without a `source_url`.
- Brief written to `briefs/<company>.md`.

## M3: Corpus

**Status: ⬜ Not started**

**Delivers:** FR-4.1–FR-4.5, FR-8.3; completes FR-6.1 (briefs now cite
comparable precedents).

Components:

- `IngestCorpus` workflow: directory walk, header-aware Markdown splitting,
  PDF extraction, content-hash idempotent upserts.
- pgvector schema and embedding client in the `research` database.
- `CorpusSearch` activity with MMR-style de-duplication.
- Briefs cite precedents from the corpus.
- Prerequisite: the explicit criteria document is written.

## M4: Loop + review

**Status: ⬜ Not started**

**Delivers:** FR-1.3, FR-2.3, FR-3.3, FR-5.1–FR-5.3, FR-5.5, FR-7.1–FR-7.3,
FR-8.1; completes FR-5.4 (per-iteration search cap), FR-9.1 (loop decisions
now visible in event history), and FR-6.4 (final manual quality validation,
with corrections feeding the corpus).

Components:

- Bounded analysis loop (`MaxIterations`, `MaxSearchesPerIteration`) with
  the `Analyze` coverage verdict.
- `review` signal and `status`/`brief` queries; signal-with-start on the
  start endpoint.
- Correction-driven brief regeneration; corrections indexed into the corpus.
- Findings archive in Postgres and `loadPriorState` reload, so evaluations
  accumulate across completed runs.

## M5: Cost + polish

**Status: ⬜ Not started**

**Delivers:** FR-9.2, FR-9.3; makes FR-5.3/FR-5.4 caps configurable per run.

Components:

- Per-run token budget that short-circuits the loop when exceeded.
- Token/cost totals in the `status` query and final workflow result.
- Namespace retention raised so past runs stay inspectable in the Web UI.
- API hardening: bearer-token middleware, localhost binding defaults.
