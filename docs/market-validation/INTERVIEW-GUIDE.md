# Phase 0/1 Customer Interview Guide

Use this guide to test a narrow commercial hypothesis: teams that need local, fail-closed AppSec feedback may pay to pilot Kuro Core in a real developer workflow. The goal is evidence of a problem, behavior, and a paid next step—not approval of the concept.

## Ideal customer profile (ICP) hypothesis

Start with teams that meet most of these conditions:

- Software engineers or security/platform engineers own repositories and developer tooling.
- They need local or self-contained security checks because source code cannot casually leave the workstation or build environment.
- They already experience a recurring cost from secrets, SAST/dependency/IaC findings, failed releases, or inconsistent pre-push/CI checks.
- A named technical owner can run a two-week pilot and a budget owner can approve a paid experiment.

Record the actual segment, repository shape, current tools, and buying authority. Do not treat “security is important” as ICP evidence.

## 30-minute interview plan

| Minutes | Objective | Prompts |
| ---: | --- | --- |
| 0–3 | Context | “What do you ship, and who owns repository security today?” “Which repos or workflows matter most?” |
| 3–10 | Recent problem | “Tell me about the last secret or security finding that interrupted work.” “What happened, who handled it, and how long did it take?” |
| 10–16 | Current workaround | “Show me the command, CI job, hook, or dashboard used today, if possible.” “Where does it fail: setup, signal quality, runtime, ownership, or enforcement?” |
| 16–21 | Reaction to the workflow | Show the synthetic demo only after the current workflow is understood. Ask: “Where would this local JSON result fit?” “What would make pass, review, and block actionable?” |
| 21–26 | Pilot design | “Which three repositories or paths would be representative?” “What baseline and success measure would convince you?” “Who must approve access, runtime, and spend?” |
| 26–30 | Commitment | “Would you run a paid, time-boxed pilot?” “What budget, owner, and start date could you commit to?” Confirm the next artifact and calendar date. |

Do not lead with attestation or the proxy. Introduce them only if the interviewee's workflow requires offline verification or a Smart-HTTP push gate. State that proxy use is optional and that this kit makes no organizational-enforcement claim.

## Evidence versus compliments

| Signal | Count as evidence when… | Do not count when… |
| --- | --- | --- |
| Problem | They describe a recent incident with impact, frequency, owner, or cost. | They agree that security “sounds important.” |
| Workflow fit | They identify a real command, repository, hook, CI job, or policy step to replace or augment. | They say it could fit “somewhere.” |
| Product value | They run the demo, supply representative fixtures, request a repeat, or ask a concrete implementation question. | They praise the UI, idea, or founder without changing behavior. |
| Pilot intent | They name repositories, an owner, decision criteria, and a date. | They say “send information” without a next step. |
| Paid validation | They discuss a budget, procurement path, paid pilot, deposit, or signed order. | They request a free trial indefinitely. |

Capture exact words, observed actions, artifacts offered, and the next commitment. Ask permission before recording repository details or outputs.

## Pilot scorecard

Agree on the scorecard before running a pilot. Use a baseline from the customer's current process where possible.

| Area | Measure | Example evidence to collect |
| --- | --- | --- |
| Setup | Time from clean machine to first JSON result; prerequisite failures | Setup log, `kuro doctor` output, runtime/image issues |
| Coverage | Representative repositories/paths scanned and scan modes used | Repo list, local scan commands, optional proxy decision |
| Signal | Pass/block/review outcomes and findings confirmed by the owner | JSON results, triage notes, false-positive/true-positive labels |
| Workflow | Time from finding to owner action; where the result is consumed | Pull request, pre-push, CI, ticket, or remediation record |
| Reliability | Completed runs versus failed/error runs; repeatability on the same fixtures | Run log, exit codes (`0` pass, `2` review, `1` block/error) |
| Adoption | Number of people who run it without coaching and number of repeat runs | Session log, commands run, follow-up usage |
| Commercial | Named buyer, budget range, paid decision, and next date | Pilot proposal, purchase path, calendar invite |

A pilot result is not “successful” merely because a scan passes. The customer must confirm that the signal changes a real workflow and that the operating prerequisites are acceptable.

## Paid-validation ask

Use a direct, bounded ask after the prospect has described a real problem:

> “Based on the workflow and repositories we discussed, would you approve a paid, two-week Kuro pilot? We would define the repositories, owner, scorecard, and success decision up front. What budget and procurement path would make that possible, and can we schedule the decision with the person who owns it?”

If they cannot buy immediately, ask for a concrete substitute that tests intent: a written pilot scope, named technical owner, representative repository access, a budget review date, or an introduction to the budget owner. Record which one they accepted and by when.

## Continuation and stop thresholds

Review after every five interviews and at the end of the first ten:

**Continue the ICP and pilot motion when all are true:**

- At least 5 interviews are with the hypothesized ICP, and at least 3 describe the same recurring problem with a recent example.
- At least 2 prospects provide a representative workflow, repository/fixture, or technical owner for a pilot.
- At least 1 prospect accepts a paid-pilot discussion with a budget owner and a dated decision step.
- Pilot evidence shows the agreed signal and reliability measures are improving or are acceptable to the owner.

**Stop, narrow, or change the hypothesis when any are true:**

- After 10 interviews, fewer than 3 ICP prospects report a recent, costly, recurring problem.
- Prospects will discuss the idea but none will provide a workflow, artifact, owner, or dated next step.
- A representative pilot cannot meet the agreed setup, signal, reliability, or data-handling requirements.
- The only perceived value depends on organizational enforcement or centralized features not present in Core.

Write the decision, evidence, and changed hypothesis after each review. Keep compliments in a separate notes field so they cannot inflate the scorecard.
