# Milestones

Tracks the milestones from the [design doc](design_doc.md), the components
each one delivers, and their status. When a milestone is done, its entry
links to the git commit that completed it (use the merge/final commit; once
the repo has a remote, turn hashes into full commit URLs).

## Process

Each milestone is delivered test-first, in two phases:

1. **Tests first.** Write the unit and integration tests that validate the
   behaviors we want to observe once the milestone is complete — before any
   implementation. These land as the milestone's first commit and require
   explicit review and approval before implementation starts. At this point
   the tests fail (or are skipped pending unbuilt components); that is the
   expected state.
2. **Implementation.** Build the milestone's components, making the relevant
   tests pass as each component lands. A component is not done until the
   tests covering it pass.

A milestone is **done** when every test from its first commit passes
unmodified. If implementation reveals that a test encoded the wrong
expectation, changing that test is its own commit with a justification —
never silently adjusted to match the implementation. The completing commit
is then linked from the table below.

| Milestone | Summary | Status |
|---|---|---|
| [M0](#m0-design) | Requirements and design docs | ✅ Done (`003ae6a`) |
| [M1](#m1-skeleton) | Compose stack, worker and api binaries, trivial workflow | ⬜ Not started |
| [M2](#m2-linear-research) | Linear research pipeline, brief to disk | ⬜ Not started |
| [M3](#m3-corpus) | Corpus ingestion and retrieval | ⬜ Not started |
| [M4](#m4-loop--review) | Analysis loop, human review, accumulation | ⬜ Not started |
| [M5](#m5-cost--polish) | Token budgets, cost reporting, hardening | ⬜ Not started |

## M0: Design

**Status: ✅ Done** — commits `4de7ded` (license), `9238dc6` (functional
requirements), `096a02d` (design doc), `003ae6a` (README).

Components:

- Functional requirements (FR-1–FR-9), technology-agnostic.
- Design doc for the Temporal/Go implementation.
- README and MIT license.

## M1: Skeleton

**Status: ⬜ Not started**

Components:

- Docker Compose file: Temporal server (Postgres persistence), Postgres 16
  with pgvector, Temporal Web UI.
- `worker/` binary registering a trivial workflow and activity.
- `api/` binary: stdlib `net/http` server with `/healthz` and one endpoint
  that starts the trivial workflow.
- One end-to-end request inspectable in the Temporal Web UI.

## M2: Linear research

**Status: ⬜ Not started**

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

Components:

- `IngestCorpus` workflow: directory walk, header-aware Markdown splitting,
  PDF extraction, content-hash idempotent upserts.
- pgvector schema and embedding client in the `research` database.
- `CorpusSearch` activity with MMR-style de-duplication.
- Briefs cite precedents from the corpus.
- Prerequisite: the explicit criteria document is written.

## M4: Loop + review

**Status: ⬜ Not started**

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

Components:

- Per-run token budget that short-circuits the loop when exceeded.
- Token/cost totals in the `status` query and final workflow result.
- Namespace retention raised so past runs stay inspectable in the Web UI.
- API hardening: bearer-token middleware, localhost binding defaults.
