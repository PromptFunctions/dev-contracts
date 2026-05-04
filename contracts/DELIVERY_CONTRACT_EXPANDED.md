<!-- CONSTANTS:START -->
<pre>
    SCOPE_CORE              = "changes limited to explicitly listed files and functions"
    GUARDRAIL_NO_REWRITE    = "no full file rewrites or refactors"
    EVIDENCE_REQUIRED       = "all claims must be backed by file paths, commands, or measurable output"
    SAFETY_DEFAULT          = "prefer reversible and low-risk changes before broad impact updates"
</pre>
<!-- CONSTANTS:END -->


# Delivery Contract (Expanded)

A structured contract for planning, implementing, validating, and handing off software changes.
Built for deterministic execution in human + agent workflows.

---

<!-- $${ -->
## CONTEXT
<!-- $$[ -->
    - Describe the objective and business/technical motivation.
    - Define current behavior and target behavior.
    - Include concrete before -> after examples.
    - Identify affected systems, services, and ownership boundaries.
    - List assumptions that must be true for the work to succeed.
    - $$(EVIDENCE_REQUIRED)
<!-- $$] -->
<!-- $$} -->

---

<!-- $${ -->
## REQUIREMENTS
<!-- $$[ -->
    - List functional requirements as testable statements.
    - List non-functional requirements (performance, reliability, security).
    - Define explicit acceptance criteria for completion.
    - Separate must-have requirements from nice-to-have requirements.
    - Define what is explicitly out of scope.
    - $$(SCOPE_CORE)
<!-- $$] -->
<!-- $$} -->

---

<!-- $${ -->
## CONSTRAINTS
<!-- $$[ -->
    - Document technical constraints (runtime, API compatibility, data contracts).
    - Document process constraints (time windows, deployment restrictions).
    - Document risk constraints (blast radius, migration safety, rollback limits).
    - Explicitly call out forbidden actions and anti-patterns.
    - $$(GUARDRAIL_NO_REWRITE)
    - $$(SAFETY_DEFAULT)
<!-- $$] -->
<!-- $$} -->

---

<!-- $${ -->
## EXECUTION_PLAN
<!-- $$[ -->
    - Break implementation into deterministic, ordered steps.
    - For each step, identify target files, symbols, and expected effect.
    - State what to keep unchanged in each touched area.
    - State what to remove only when required and justify it.
    - State what to replace and the replacement invariant.
    - Include checkpoints where correctness is verified before continuing.
    - Keep changes small, reviewable, and incremental.
    - $$(SCOPE_CORE)
<!-- $$] -->
<!-- $$} -->

---

<!-- $${ -->
## VALIDATION_PLAN
<!-- $$[ -->
    - Define unit/integration/e2e checks relevant to this change.
    - Define exact commands and expected success patterns.
    - Include negative tests and failure-path behavior.
    - Include deterministic output checks and ordering checks when applicable.
    - Include regression checks for adjacent components.
    - Include observability checks (logs/metrics/traces) when production-facing.
    - $$(EVIDENCE_REQUIRED)
<!-- $$] -->
<!-- $$} -->

---

<!-- $${ -->
## ROLLBACK_AND_RECOVERY
<!-- $$[ -->
    - Define rollback trigger conditions.
    - Define rollback procedure step-by-step.
    - Define data recovery or reconciliation actions if partial failure occurs.
    - Define how to verify system health after rollback.
    - Keep rollback path validated before deployment when possible.
    - $$(SAFETY_DEFAULT)
<!-- $$] -->
<!-- $$} -->

---

<!-- $${ -->
## HANDOFF
<!-- $$[ -->
    - Summarize what changed, why, and where.
    - Provide verification evidence and unresolved risks.
    - Provide operational notes for on-call/support teams.
    - Provide follow-up tasks and ownership.
    - Include exact artifacts produced (reports, logs, diff references).
    - $$(EVIDENCE_REQUIRED)
<!-- $$] -->
<!-- $$} -->
