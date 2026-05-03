# IRSEV Framework

A minimal, structured prompt framework for guiding code changes with precision.
Should be iterative-friendly in mind and promote efficient back-and-forth.

---

## ISSUE
- Describe the objective, change request, or observed problem.
- Use concrete examples when applicable (before → after).
- No speculation, only defined intent or observed behavior.
- minimal reproduction steps or expected usage scenario
- affected components / modules
- scope (CONSTANTS)
- guardrails (CONSTANTS)

---

## ROOT CAUSE
- Identify the specific mechanism causing the issue.
- Point to the exact function / logic responsible.
- Explain *why* the current behavior happens.
- file paths and line references
- conditions under which the issue manifests

---

## SOLUTION
- Define the intended behavior clearly.
- State the rule or invariant to enforce.
- Do not describe implementation yet.
- non-goals / explicitly unchanged behavior
- expected input → output mapping examples

---

## EXECUTION
- Provide precise code-level actions.
- Target exact lines / blocks to modify.
- Use:
  - ✔️ what to keep
  - ❌ what to remove
  - ➡️ what to replace with
- Avoid broad rewrites.
- step-by-step execution sequence
- exact symbols / identifiers to match

---

## VALIDATION
- List concrete checks:
  - visual outputs
  - edge cases
  - regressions to avoid
- Include exact expected patterns.
- expected logs / return values
- idempotency checks
- failure scenarios and expected behavior
- scope (CONSTANTS) -> MUST BE EXACTLY THE SAME AS ISSUE LEVEL SCOPE
- guardrails (CONSTANTS) -> MUST BE EXACTLY THE SAME AS ISSUE LEVEL GUARDRAILS