---
name: sdt-security
description: >
  Use this skill when the question is whether something is safe: auditing a
  change, a surface, a dependency or an agent that acts on the project's behalf,
  writing a threat model, reviewing secrets handling, or judging whether a
  suspected vulnerability is real — even when the request does not mention
  "security", "audit" or "threat" explicitly. It routes to the project's security
  instruction file; it does not replace it.
---

<!-- sdt:begin:skills/sdt-security/SKILL -->

# SDT security skill

Read the project's security instruction file before answering "is this safe?";
this skill routes to it and never restates it.

1. Read `context/instructions/security.md` **before any audit claim** — the
   boundary-and-result gate, the severity anchors and the three verdicts.
2. Read the project's architecture document for the surface under audit: what it
   exposes, on which bind, after which act of the user, from which origin.
3. Read `context/instructions/development.md` when the audit needs a tool: a
   scanner is chosen library-first and its output is evidence, never a verdict.
4. For an interface or a change rather than a boundary, `ui-review.md` owns the
   review procedure; this skill owns the trust question.

A finding names the principal, the boundary, the control and the observed result.
Without all four it is a hypothesis, and it is written as one.

<!-- sdt:end:skills/sdt-security/SKILL -->
