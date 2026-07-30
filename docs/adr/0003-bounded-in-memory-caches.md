# ADR 0003: Bounded in-memory caches

- Status: Accepted
- Date: 2026-07-30

## Context

Public GitHub reads are rate-limited and some analysis fan-out is expensive,
but the anonymous core must not require persistent storage or retain long-lived
user behavior.

## Decision

Use application cache ports with concurrency-safe TTL/LRU in-memory adapters,
fixed capacity, canonical non-secret keys, deep-copy boundaries, and
singleflight for equal misses. Use 30 minutes for profile analysis and five
minutes for issue candidate and detail snapshots by default.

## Consequences

Restarts clear anonymous data and horizontal instances do not share cache
state. Capacity and lifetime are explicit and validated. A future distributed
cache may implement the same ports, but cannot change anonymous persistence or
key privacy without a new decision.
