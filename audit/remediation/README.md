# Audit remediation plans

Phased plans for closing the 15 High and 51 Medium findings from the 2026-09-11 audit
(`audit/summary.md`, `audit/phase-1-architecture.md` … `audit/phase-6-security.md`,
`docs/20260911-audit-findings.md`). The prompt used to generate these plans is in
[`PROMPT.md`](PROMPT.md).

## Phases

| # | Plan | Branch | Theme | High | Medium |
|---|---|---|---|---|---|
| 1 | [phase-1.md](phase-1.md) | `audit-review-phase1` | Deployment & config security | A5-01, A5-02 | A5-03, A5-04 |
| 2 | [phase-2.md](phase-2.md) | `audit-review-phase2` | Auth & authorization | A6-01, A2-02 | A6-02, A6-03, A6-04, A6-06 |
| 3 | [phase-3.md](phase-3.md) | `audit-review-phase3` | Error contract & silent-success | A3-01, A1-02, A4-03 | A3-04, A2-08 |
| 4 | [phase-4.md](phase-4.md) | `audit-review-phase4` | Concurrency, atomicity & tx seam | A2-01, A1-03, A4-01 | A3-05, A3-06, A3-07, A1-06, A3-08, A3-12, A4-10 |
| 5 | [phase-5.md](phase-5.md) | `audit-review-phase5` | Recipe-import / OCR pipeline | A3-02, A3-03, A4-02 | A3-09, A1-13, A3-10, A3-13, A6-07, A6-08, A2-09 |
| 6 | [phase-6.md](phase-6.md) | `audit-review-phase6` | Architecture boundaries & grocery feature | A1-04, A1-01 | A3-11, A1-05, A1-07, A1-09, A1-12, A2-06 |
| 7 | [phase-7.md](phase-7.md) | `audit-review-phase7` | BFF robustness & DoS controls | — | A2-03, A2-04, A2-05, A2-07, A2-10, A6-05, A2-11, A2-12, A1-08, A1-10, A1-11 (+ Lows A3-21, A6-11 co-located) |
| 8 | [phase-8.md](phase-8.md) | `audit-review-phase8` | CI/Docker hygiene & test coverage | — | A5-05, A5-06, A5-07, A5-08, A5-09, A5-10, A4-04, A4-05, A4-06, A4-07, A4-08, A4-09 |

Totals: 15 High + 51 Medium, each assigned to exactly one phase. Corrections made while verifying the
roadmap against the reports (each is also noted in the relevant phase file):

- **Added (Medium, missing from the roadmap):** A1-06 → Phase 4 (same defect as A3-07); A1-13 → Phase 5
  (same defect as A3-09); A3-11 → Phase 6 (same defect as A1-04); A4-05 → Phase 8 (prerequisite of
  A4-04).
- **Severity corrected:** A3-21 and A6-11 were listed as Medium in the roadmap but are **Low** in the
  reports; they remain in Phase 7 as co-located Lows and do not count toward the Medium total.

## Workflow (approval-gated, one PR per phase)

1. Cut `audit-review-phaseN` from the latest `main` (which includes the previous phase's merged PR).
2. Work only the findings listed in `phase-N.md`. Lows are touched only under the co-located-Low
   policy: *fix a Low finding only if it lives in code already being changed for a High/Medium finding in
   this phase; never edit code solely to fix a Low.*
3. `go build ./...` and `go test ./...` must pass (plus `golangci-lint run ./...` / `go vet ./...`), and
   the phase's manual checks must be performed.
4. Open **one** PR from `audit-review-phaseN` into `main`, then **stop**. Do not start phase N+1 until
   that PR is reviewed and approved (per `AGENTS.md`: no direct pushes to `main`; merge only after build,
   tests and lint are verified).

## Why the phases are ordered this way

Remediation phase numbers intentionally **do not** mirror the audit's own phase numbers (audit phases are
grouped by review area — architecture, BFF, domains, tests, deploy, security; remediation phases are
grouped by the change that fixes them and by dependency order).

Sequencing rationale: Phase 3 typed errors unblock Phase 4's tx seam, which is a prerequisite for
Phase 5's atomic recipe-import Approve; test fixes fold into the phase that touches their code.
Phases 1–2 come first because they are the highest-impact security/deployment fixes and are independent
of the error-contract work; Phase 6 needs Phase 4's unit of work for grocery generation; Phases 7–8 are
hygiene and coverage that are cheaper once the code they harden or test has settled.
