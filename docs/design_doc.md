# Design Doc: Company Research Agent on Temporal (Go)

**Author:** Francesc Campoy Flores
**Status:** Draft
**Last updated:** 2026-06-11
**Implements:** [functional_requirements.md](functional_requirements.md)
**Alternative to:** the original LangGraph/Python design (since removed)

## Summary

An implementation of the company research agent as a Temporal workflow written
with the Temporal Go SDK, exposed as a REST/JSON API. Each company maps to one
durable workflow execution: research, corpus retrieval, analysis, brief
generation, and human review all happen inside a single workflow whose state
Temporal persists automatically. LLM and search calls go to hosted paid
services through open-source client libraries; everything that runs locally —
Temporal and Postgres with pgvector — is open source and managed with Docker
Compose.

The orchestration model is deliberately different from the LangGraph design:
instead of a state graph with conditional edges and an external checkpointer,
the research loop is ordinary Go control flow, and durability comes from
Temporal's event-sourced workflow history. There is no separate state schema
to checkpoint — local variables in the workflow function *are* the state.

## Requirements Mapping

This design must satisfy every functional requirement in
[functional_requirements.md](functional_requirements.md). The short version
of how:

| Requirement | Mechanism |
|---|---|
| FR-1 run initiation/resumption | One workflow per company, ID derived from the normalized name; signals resume paused runs, prior findings reload from Postgres |
| FR-2 research planning | `PlanResearch` activity (LLM call) inside the workflow loop |
| FR-3 web research | `WebSearch` + `FetchPage` + `ExtractFindings` activities; findings appended to a workflow-local slice |
| FR-4 corpus retrieval | `CorpusSearch` activity over pgvector; ingestion workflow keeps the corpus current |
| FR-5 analysis loop | A Go `for` loop bounded by `MaxIterations`; `Analyze` activity decides continue vs. finish |
| FR-6 brief generation | `GenerateBrief` activity with schema-constrained LLM output into a Go struct |
| FR-7 human review | Workflow blocks on a Temporal signal; API endpoints deliver approve/corrections |
| FR-8 persistence | Temporal history for run state; `PersistBrief` activity writes Markdown and indexes into pgvector |
| FR-9 observability/cost | Temporal Web UI event history per run; token/cost totals aggregated in workflow state, exposed via query and in the result |

## Architecture Overview

Three processes, one Compose file:

```
                +-------------------+
   HTTP/JSON    |  api (Go, stdlib) |----------+
  ------------> |  net/http server  |          | Temporal client
                +-------------------+          | (start, signal, query)
                                               v
                +-------------------+    +-----------------+
                |  worker (Go)      |<-->|  Temporal server |
                |  workflows +      |    |  (OSS, Postgres  |
                |  activities       |    |   persistence)   |
                +-------------------+    +-----------------+
                   |        |
        LLM / search APIs   |  pgvector queries, brief files
        (hosted, paid)      v
                +-------------------+
                |  Postgres 16      |
                |  + pgvector       |
                +-------------------+
```

- **api**: a stdlib `net/http` server translating REST calls into Temporal
  client operations (start workflow, signal, query). It holds no state.
- **worker**: hosts the workflow and activity implementations. All
  non-deterministic work (LLM calls, HTTP fetches, DB queries, file writes)
  lives in activities; the workflow function contains only orchestration
  logic, as Temporal's determinism rules require.
- **Temporal server**: the open-source self-hosted distribution, using
  Postgres for its own persistence. Its Web UI comes along for free and shows
  every run's full event history.
- **Postgres**: one instance, three databases — `temporal`,
  `temporal_visibility`, and `research` (corpus embeddings via pgvector,
  finding archive, run metadata).

## The Research Workflow

One workflow type, `CompanyResearch`, with workflow ID
`research-<normalized-company-slug>` (FR-1.2). Sketch:

```go
func CompanyResearch(ctx workflow.Context, in ResearchInput) (*CompanyBrief, error) {
    state := loadPriorState(ctx, in)        // activity: prior findings from Postgres (FR-1.3)

    for state.Iteration < in.MaxIterations { // FR-5.3, default 3
        plan := execActivity[Plan](ctx, a.PlanResearch, state)          // FR-2
        results := execActivity[[]SearchResult](ctx, a.WebResearch,
            plan, in.MaxSearchesPerIteration)                           // FR-3, FR-5.4
        findings := execActivity[[]Finding](ctx, a.ExtractFindings, results)
        state.Findings = append(state.Findings, findings...)            // FR-3.3 append-only

        state.Corpus = execActivity[[]Excerpt](ctx, a.CorpusSearch, state) // FR-4

        verdict := execActivity[Verdict](ctx, a.Analyze, state)         // FR-5.1
        state.Tokens.Add(verdict.Usage)
        if verdict.Sufficient {
            break
        }
        state.Iteration++                                               // FR-5.2
    }

    brief := execActivity[CompanyBrief](ctx, a.GenerateBrief, state)    // FR-6

    for {                                                               // FR-7
        decision := waitForReviewSignal(ctx)            // blocks durably
        if decision.Approved {
            break
        }
        state.Corrections = append(state.Corrections, decision.Notes)
        brief = execActivity[CompanyBrief](ctx, a.GenerateBrief, state) // regenerate
    }

    execActivity(ctx, a.PersistBrief, brief, state.Corrections)         // FR-8.2, FR-8.3, FR-7.3
    return &brief, nil
}
```

Notable properties this buys over the LangGraph design:

- **Durability is implicit.** If the worker dies mid-run, Temporal replays
  the event history and the workflow resumes at the exact statement it was
  on, including while blocked on human review for days. There is no
  checkpointer to configure and no state schema to keep in sync (FR-8.1).
- **The loop is just a loop.** Conditional edges become an `if` statement;
  the iteration cap is a `for` condition. Control flow is debuggable with a
  debugger.
- **Retries are declarative.** Each activity gets a retry policy
  (e.g. 3 attempts, exponential backoff, non-retryable on 4xx). Flaky search
  APIs and LLM rate limits are handled by the platform, not by hand-rolled
  retry code.

### Queries and Signals

- **Query `status`**: returns iteration count, finding count, token totals,
  and current phase. Backing for `GET /runs/{id}`.
- **Query `brief`**: returns the draft brief while the workflow waits for
  review (FR-7.1).
- **Signal `review`**: carries `{approved: bool, notes: []string}` (FR-7.2).
- **Signal-with-start** is used by the start endpoint so that "start or
  resume" is a single race-free call: if the workflow exists it is signalled,
  otherwise it is started (FR-1.3).

A finished workflow's findings are archived to the `research` database by
`PersistBrief`, so a later re-evaluation of the same company starts a fresh
workflow execution (new run ID, same workflow ID series) that seeds itself
from the archive in `loadPriorState`. Accumulation across sessions therefore
survives workflow completion, not just interruption.

## Activities

| Activity | Calls | Notes |
|---|---|---|
| `PlanResearch` | LLM | Produces/updates open questions; initial plan covers funding, founders, product, competitors, hiring signals (FR-2.2) |
| `WebResearch` | Search API + HTTP fetch | Capped searches per iteration; readability extraction on fetched pages |
| `ExtractFindings` | LLM (cheap model) | Schema-constrained output into `[]Finding`; rejects findings without a `source_url` (FR-3.4) |
| `CorpusSearch` | Postgres/pgvector | Embeds queries (company domain, product category, risks), MMR-style de-duplication for diversity (FR-4.4) |
| `Analyze` | LLM (strong model) | Coverage verdict; may call `CorpusSearch` results already in state or request precedent lookups (FR-5.5) |
| `GenerateBrief` | LLM (strong model) | Emits the full `CompanyBrief` struct with citations (FR-6) |
| `PersistBrief` | Postgres + filesystem | Writes `briefs/<company>.md`, embeds and indexes the brief and any corrections into pgvector (FR-8.2, FR-8.3) |
| `IngestDocument` | Postgres + filesystem | Used by the corpus ingestion workflow below |

Activities are plain Go functions taking and returning JSON-serializable
structs. `CompanyBrief`, `Finding`, etc. are Go structs with `json` tags —
the schema lives in one place and is enforced both at the LLM boundary
(structured output) and at the API boundary (`encoding/json`).

### LLM access

Hosted models through the official open-source Go SDK (the Anthropic Go SDK
is MIT-licensed). Two model tiers behind a small internal interface
(FR-9.4):

- **extract tier** (cheap): `ExtractFindings`, embedding-adjacent chores.
- **reason tier** (strong): `Analyze`, `GenerateBrief`, `PlanResearch`.

Structured output uses tool-use/JSON-schema constraints derived from the Go
struct definitions, with `encoding/json` + validation as the backstop. Every
activity result includes token usage, which the workflow aggregates (FR-9.3).

### Web search

**Tavily** (chosen for M2: built for LLM research agents, returns cleaned
relevance-scored content, free tier covers this tool's volume) behind a
one-method `Searcher` interface so the provider is swappable. Only the
client code is in-repo; the service itself is the one deliberate
non-open-source dependency alongside the LLM, per the decision to keep the
*stack* open source while consuming hosted intelligence and search.

### Testing strategy

Hermetic by default: unit and integration tests use fake LLM and search
implementations behind the `llm.Client` and `search.Searcher` interfaces, so
PR CI is deterministic, free, and needs no secrets. A separate suite (build
tag `live`, `make test-live`) hits the real Tavily and Anthropic APIs with
deliberately loose assertions — it exists to catch API drift, not to grade
model output — and runs on a weekly schedule plus manual dispatch, like
govulncheck, so drift is caught without making merges depend on external
services.

## Corpus Ingestion

A second, simpler workflow: `IngestCorpus` walks the corpus directory
(resume, criteria doc, exported notes, past briefs), splits documents with a
header-aware Markdown splitter so risk/criteria sections stay coherent
(FR-4.5), embeds chunks, and upserts them into pgvector keyed by content
hash — re-running it is idempotent. Triggered via the API or on a Temporal
cron schedule. PDF extraction uses an open-source Go library
(e.g. `pdfcpu`/`ledongthuc/pdf`); chunking and MMR are small amounts of
first-party code rather than a framework dependency — at this scale they are
~100 lines each, and writing them is part of the point of the Go rewrite.

Embeddings come from a hosted embedding API through its open-source client,
mirroring the LLM decision; one model for the whole corpus, since consistency
matters more than the specific choice.

## API

Stdlib `net/http` with `ServeMux` patterns, JSON in/out, no framework:

| Method & path | Effect |
|---|---|
| `POST /companies/{name}/runs` | Signal-with-start the `CompanyResearch` workflow (FR-1.1, FR-1.4) |
| `GET /companies/{name}/runs/current` | Workflow `status` query: phase, iteration, findings count, token totals |
| `GET /companies/{name}/brief` | Draft brief (via query) while under review; final brief from disk after approval |
| `POST /companies/{name}/review` | Send the `review` signal with `{approved, notes}` (FR-7.2) |
| `POST /corpus/ingest` | Start `IngestCorpus` |
| `GET /healthz` | Liveness |

Single-user tool, so auth is a static bearer token checked by middleware;
the server binds to localhost by default.

## Observability

Temporal's built-in observability only — no extra services and no extra
storage beyond the Postgres the server already uses:

- **Temporal Web UI.** Ships with the standard self-hosted Compose setup and
  shows the authoritative event history for every run: each activity's full
  input and output, every retry, timings, and what the workflow is currently
  blocked on. Because activity payloads include plans, findings, analysis
  verdicts, and token usage, loop decisions and retrieval quality are
  readable directly off the history (FR-9.1).
- **Run metadata and cost in workflow state.** The `status` query returns
  company, phase, iteration count, finding count, and aggregated token/cost
  totals at any moment; the same totals are part of the final workflow result
  (FR-9.2, FR-9.3). No metrics pipeline needed — the state *is* the report.
- **Logs.** Structured `slog` JSON from api and worker to stdout, captured by
  Docker; `docker compose logs` filtered by workflow ID covers debugging.

Storage details: event history lives in the `temporal` database and workflow
listing/filtering in `temporal_visibility` — both in the existing Postgres
instance. Postgres-backed visibility supports search attributes directly, so
no Elasticsearch is required. The one knob to turn: the namespace retention
period (default 72 hours after a run closes) should be raised — e.g. to 90
days — so past runs stay inspectable in the UI. Durable artifacts (briefs on
disk, the findings archive in the `research` database) outlive retention
regardless, so nothing of record is lost when history ages out.

The Temporal server and Go SDK also expose Prometheus-format metrics; nothing
scrapes them in this design, but bolting a collector on later requires no
code changes.

## Tech Stack

All locally-run components open source:

- **Go** (latest stable), modules; no web or LLM framework.
- **Temporal OSS** self-hosted (MIT), Go SDK (`go.temporal.io/sdk`, MIT).
- **Postgres 16 + pgvector** (PostgreSQL licence) — Temporal persistence and
  application data in one instance.
- **Docker Compose** to run all of the above.
- Hosted external services (the two deliberate exceptions): LLM + embeddings
  API via its open-source Go SDK, and a paid web-search API.

## Cost Controls

- `MaxIterations = 3` and `MaxSearchesPerIteration = 5` as workflow input
  defaults (FR-5.3, FR-5.4), overridable per run via the start request.
- Cheap model tier for extraction, strong tier only for plan/analyze/brief.
- Token usage flows back from every LLM activity; the workflow keeps a
  running total, exposes it via the `status` query, and includes it in the
  final result.
- A per-run token budget: if the running total exceeds it, the workflow
  skips further iterations and proceeds to brief generation with what it has.

## Milestones

Tracked in [milestones.md](milestones.md): per-milestone components, current
status, and the test-first delivery process. Each milestone is independently
usable and demoable.

## Risks and Open Questions

- **Workflow history growth.** Findings and corpus excerpts pass through
  workflow state; large payloads bloat event history. Mitigation: activities
  return findings, but bulky raw page content stays activity-internal; if
  payloads still grow, store them in Postgres and pass references. Temporal's
  2MB payload limit is the hard backstop.
- **Determinism discipline.** All randomness, time, and I/O must go through
  the SDK or activities. The `workflowcheck` static analyzer runs in CI.
- **Versioning long-running workflows.** A run blocked on human review for a
  week must survive a code deploy. Use Temporal's versioning/patching API for
  any change to workflow code; activity changes are safe.
- **Structured output without a framework.** Schema-constrained LLM output in
  Go means maintaining JSON schemas alongside structs. Mitigation: generate
  schemas from struct tags with an open-source generator
  (e.g. `invopop/jsonschema`) so there is one source of truth.
- **Search/LLM as closed services.** Accepted explicitly: the stack is open
  source; the intelligence is rented. Swappable interfaces keep the door open
  for SearXNG or local models later.

## Alternatives Considered

- **LangGraph/Python**: the original design. Stronger
  ecosystem of loaders/retrievers; weaker durability story (checkpointer vs.
  event sourcing) and a framework API-churn risk that Temporal does not have.
- **Plain Go with a Postgres state table.** Fewer moving parts than Temporal,
  but hand-rolls resumability, retries, and the days-long human-review pause
  that Temporal gives for free — exactly the hard parts.
- **River/Asynq job queues.** Good for fire-and-forget jobs, but the research
  loop is a stateful multi-step saga with a human in the middle; job queues
  push that state back into hand-written DB code.
