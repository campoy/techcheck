# CLAUDE.md

Project: **techcheck**, a company research agent on Temporal with the Go SDK.
Start with the docs — they are the source of truth for what to build and in
what order.

## Documents

- `docs/functional_requirements.md` — what the system does. Keep it strictly
  technology-agnostic: behaviors only, never specific technologies,
  frameworks, or products.
- `docs/design_doc.md` — how it's built (Temporal, Go, Postgres/pgvector).
- `docs/milestones.md` — the **single source of truth** for milestones, their
  components, FR mapping, and status. Do not duplicate milestone lists in
  other documents; link to this one instead.
- Docs live in `docs/` with lowercase snake_case filenames.

## Design constraints

- The system is exposed as an **API**, not a CLI.
- Everything that runs locally must be **open source**. The two deliberate
  exceptions are hosted services accessed through open-source clients: the
  LLM/embeddings API and a paid web-search API.
- Observability comes from Temporal's own Web UI, event history, and workflow
  state — no additional observability services or storage.
- License is MIT.

## Process

- **Test-first milestones, one PR per milestone.** Each milestone is a
  single PR whose first commit contains the unit and integration tests that
  encode its behaviors — the FR IDs on its "Delivers" line in
  `docs/milestones.md` are the checklist. That commit requires explicit user
  review and approval (as PR review, while checks are still red) before
  implementation commits join the same PR; the PR merges once green.
  Implementation then makes those tests pass; a milestone is done only when
  all of its first-commit tests pass unmodified. If a test encoded a wrong
  expectation, fix it in its own commit with a justification — never adjust
  it silently.
- **Milestone bookkeeping.** When a milestone completes, update its status in
  `docs/milestones.md` and link the completing commit; for M1 onward, that
  doc update rides inside the completing commit itself.
- **Keep the FR mapping current.** If requirements or milestone scope change,
  update the Delivers lines so every FR remains assigned and partials point
  to the milestone that finishes them.

## Working style

- **All changes to `main` go through pull requests** — never commit to main
  directly. Work on a branch, push, open a PR with `gh`, and let the user
  review and merge. A repository ruleset enforces this server-side. The
  test-first milestone commits get their required approval as PR review.
- **The repo is public.** Personal data — evaluation briefs, the corpus,
  criteria documents — must never be committed; `briefs/` and `corpus/` are
  gitignored for that reason.
- Ask for clarification on design and preference decisions instead of
  assuming; for multi-step git operations, propose a plan and wait for
  approval before executing.
- Small, focused commits — one logical change each — with conventional-commit
  prefixes (`docs:`, `chore:`, `feat:`, `test:`, `fix:`).
- After a substantial change, ask whether it should be committed; never
  commit without confirmation.
- Commit messages must not reference Claude or Anthropic in any form — no
  Co-Authored-By trailers, no "generated with" lines.
