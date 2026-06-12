# Functional Requirements: Company Research Agent

**Author:** Francesc Campoy Flores
**Status:** Draft
**Last updated:** 2026-06-11
**Derived from:** the original LangGraph design doc, since removed (technology-specific details stripped)

## Purpose

A system that, given a company name, researches the company and produces a
structured evaluation brief tailored to the user's job search criteria. It
combines web research with retrieval over a personal corpus (resume, past
company evaluations, explicit criteria) and persists its state so research can
be paused, resumed, and accumulated over time.

## Scope

In scope: research, retrieval, analysis, brief generation, human review, and
persistence for a single user, exposed as an API.

Out of scope:

- Production-grade reliability or multi-user support.
- Automated outreach, application submission, or any action beyond research.
- A graphical user interface in v1.
- Scraping sources that block automated access (e.g. LinkedIn). The system
  works with what public search and fetching can reach; anything else the user
  adds manually to the corpus.

## Functional Requirements

### FR-1: Run initiation and resumption

- FR-1.1: The system shall accept a company name as input and start a research
  run for that company.
- FR-1.2: The system shall normalize the company name and map each company to
  a stable identifier.
- FR-1.3: If prior research exists for a company, re-running the system on
  that company shall resume from the accumulated state rather than restart
  from scratch.
- FR-1.4: The system shall be exposed as an API, with operations to start a
  new run and to resume an existing one.

### FR-2: Research planning

- FR-2.1: The system shall produce a research plan as a list of open questions
  to answer about the company.
- FR-2.2: The initial plan shall cover at minimum: funding history and
  investors, founders and team background, product and differentiation,
  competitors and commoditization risk, and hiring signals for Staff/Principal
  individual-contributor roles.
- FR-2.3: The system shall update the plan as findings accumulate, removing
  answered questions and adding new ones that emerge.

### FR-3: Web research

- FR-3.1: The system shall search the public web and fetch web pages to answer
  the open questions in the research plan.
- FR-3.2: Each research result shall be parsed into a structured finding with:
  source URL, claim, category (funding, team, product, market, or risk), and a
  confidence level.
- FR-3.3: Findings shall be accumulated append-only across iterations and
  sessions; later research shall not silently overwrite earlier findings.
- FR-3.4: Every finding must carry a source URL; claims without a source shall
  not be recorded as findings.

### FR-4: Personal corpus retrieval

- FR-4.1: The system shall maintain a searchable personal corpus comprising:
  the user's resume, past evaluation briefs, an explicit hand-written criteria
  document (values, stage, and role requirements), and selected manually
  exported notes.
- FR-4.2: During a run, the system shall retrieve corpus content relevant to
  the company under evaluation, using the company's domain, product category,
  and identified risks as retrieval cues.
- FR-4.3: Retrieval shall surface applicable precedents from past evaluations
  (e.g. a prior values-based rule-out or stage-mismatch decision) so they can
  inform the current evaluation.
- FR-4.4: Retrieved results shall be diverse: the system shall avoid returning
  multiple near-duplicate excerpts from the same document when broader
  coverage is available.
- FR-4.5: Documents shall be divided for indexing in a way that keeps
  logically coherent sections (e.g. the risks or criteria section of a brief)
  together.

### FR-5: Analysis and iteration

- FR-5.1: The system shall evaluate whether accumulated findings sufficiently
  answer the research plan.
- FR-5.2: If coverage is insufficient, the system shall loop back to update
  the plan and perform additional research.
- FR-5.3: The research loop shall be bounded by a maximum iteration count
  (default 3); when the cap is reached the system shall proceed to brief
  generation with the findings it has.
- FR-5.4: The number of web searches per iteration shall be capped
  (default 5).
- FR-5.5: The analysis step shall be able to query the personal corpus on
  demand to pull precedents while reasoning.

### FR-6: Brief generation

- FR-6.1: The system shall produce a structured evaluation brief containing:
  - company name and a one-line description;
  - a funding summary;
  - a team summary;
  - a product assessment;
  - a fit score from 1 to 10, assigned against a defined rubric, with a
    written rationale;
  - values flags (e.g. defense contracts, surveillance);
  - a stage assessment;
  - a list of risks;
  - a list of questions for the user to ask the company;
  - comparable precedents from past evaluations (e.g. "similar stage concern
    as TribeROI");
  - the list of sources used.
- FR-6.2: Claims in the brief shall cite their sources.
- FR-6.3: Weakly sourced claims shall be flagged via the finding confidence
  level.
- FR-6.4: Briefs shall be comparable in quality and structure to the user's
  manually written evaluations, applying the same criteria consistently to
  every company.

### FR-7: Human review

- FR-7.1: Before a brief is finalized, the system shall pause and present it
  to the user for review.
- FR-7.2: The user shall be able to approve the brief or annotate corrections.
- FR-7.3: Corrections shall be written back into the run's state and into the
  personal corpus, so the user's judgment informs future evaluations.

### FR-8: Persistence

- FR-8.1: The system shall checkpoint run state per company so a run can be
  paused and resumed across sessions, with findings accumulating over time.
- FR-8.2: Approved briefs shall be written to disk as Markdown files, one per
  company.
- FR-8.3: Approved briefs shall be indexed into the personal corpus so future
  evaluations can retrieve them as precedents.
- FR-8.4: Data retention is manual; no automated cleanup is required.

### FR-9: Observability and cost reporting

- FR-9.1: All runs shall be traced with enough detail to understand agent
  behavior, including loop decisions and retrieval quality, not only failures.
- FR-9.2: Each run shall record metadata including company name, iteration
  count, and total token usage.
- FR-9.3: The system shall log token usage and cost per run and report the
  total at the end of each run.
- FR-9.4: The system shall support using a cheaper model for high-volume
  extraction work and a stronger model for analysis and brief generation.

## Prerequisites and Constraints

- The explicit criteria document must exist before corpus retrieval is built;
  without it, retrieval has little to anchor on.
- A small monthly budget for web search may be required; free tiers may be too
  rate-limited.

## Acceptance Examples

The system must be able to encode and reapply precedents such as:

- **Scale AI**: ruled out on values grounds (military and surveillance
  contracts) — should surface as a values precedent when evaluating companies
  in similar spaces.
- **TribeROI**: ruled out as a stage mismatch — should surface as a stage
  precedent for similarly staged companies.
