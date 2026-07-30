# ADR 0005: Docker-free native delivery

- Status: Accepted
- Date: 2026-07-30

## Context

IssueScout needs portable, verifiable delivery without a selected cloud
provider. The project explicitly excludes Docker, OCI images, container
actions, and a container-runtime dependency.

## Decision

Cross-compile static Go API executables and package the Vite build as
deterministic tar/gzip archives. Normalize timestamps, ordering, ownership,
modes, build IDs, and gzip metadata; publish checksums, manifests, SBOM, and
attestations. Promotion consumes verified archives through a provider-neutral
contract.

## Consequences

The same source and version must build byte-identical output. CI builds twice,
scans extracted secret surfaces, verifies request correlation and graceful
shutdown, and rejects container configuration. Provider integration must adapt
to these native artifacts rather than redefining the release format.
