# ADR 0004: Deterministic explainable scoring

- Status: Accepted
- Date: 2026-07-30

## Context

Contributors need to understand why work is recommended. A probabilistic or
opaque score could fabricate certainty from incomplete GitHub samples and
would be difficult to reproduce.

## Decision

Use pure bounded rules, typed evidence and confidence, conservative unknown
states, fixed score components totaling 100, and stable tie-breakers. List and
detail use the same analysis and recommendation service.

## Consequences

Identical normalized evidence yields the same result and can be exhaustively
tested. Rule expansion requires explicit tables and regression cases. The
model is intentionally a decision aid rather than a success prediction.
