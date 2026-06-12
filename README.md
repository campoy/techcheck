# techcheck

A company research agent for an active job search. Given a company name, it
researches the company on the public web, retrieves relevant precedents from a
personal corpus (resume, explicit criteria, past evaluations), and produces a
structured evaluation brief — funding, team, product assessment, fit score,
values flags, risks, and questions to ask — with every claim cited.

Evaluations accumulate: approved briefs are indexed back into the corpus, so
past decisions (a values-based rule-out, a stage mismatch) surface as
precedents when evaluating the next company.

**Status: design phase.** No code yet.

## Documentation

- [Functional requirements](docs/functional_requirements.md) — what the
  system must do, technology-agnostic.
- [Design doc](docs/design_doc.md) — the proposed implementation on Temporal
  with the Go SDK.
- [Milestones](docs/milestones.md) — delivery plan, per-milestone components,
  and status.

## Design at a glance

- **Orchestration**: one durable [Temporal](https://temporal.io) workflow per
  company. The research loop is plain Go control flow; durability, retries,
  and resumption come from Temporal's event-sourced history.
- **Interface**: a REST/JSON API (Go stdlib `net/http`) to start or resume a
  run, check status, read the draft brief, and approve or correct it. Human
  review is a Temporal signal the workflow blocks on.
- **Retrieval**: personal corpus embedded into Postgres + pgvector — the same
  Postgres instance that backs Temporal.
- **Intelligence**: hosted LLM and web-search APIs, accessed through
  open-source client libraries. Everything that runs locally is open source,
  managed with Docker Compose.
- **Observability**: Temporal's Web UI and event history; token and cost
  totals tracked in workflow state and reported per run. No extra services.

## Planned layout

```
docs/       requirements and design
api/        REST server (Temporal client)
worker/     workflow + activity implementations
briefs/     approved evaluation briefs (Markdown)
corpus/     personal corpus sources
```

## License

[MIT](LICENSE).
