# Proposed architecture

## W1 review draft

The concrete [W1 system design](w1-system-design.md),
[DynamoDB model](w1-data-model.md), and
[decision sheet](w1-review-decisions.md) are proposed for Davian Hernandez's
review. They distinguish repository foundations and completed spike evidence
from planned product behavior. They do not change accepted contracts or ADRs
and do not authorize deployment or implementation.

Kurier's architecture is a planning baseline. It should evolve when spikes,
security review, or vertical-slice delivery provide better evidence.

## Components

- A React and TypeScript web application presents request authoring, execution
  status, and sanitized evidence.
- A Go API handles authenticated application requests and persists durable
  metadata in DynamoDB through proposed Lambda handlers.
- A Go execution worker consumes queued jobs, performs remote API requests, and
  writes immutable, deliberately redacted evidence.
- A Go local agent will execute requests that require access to a developer's
  machine or private network. A future MCP interface will expose only
  deliberately sanitized evidence to coding agents.

The revised W1 topology uses CloudFront for web delivery, Go Lambda handlers
behind HTTP API Gateway, DynamoDB metadata/protected envelopes, SQS execution
wakeups, and private S3 sanitized evidence. It is recorded in
[proposed ADR 0002](../decisions/0002-serverless-persistence-topology.md), not
deployed or runtime-tested. SST 4 with TypeScript is the
selected infrastructure framework following the compute/frontend and database
viability spikes. GitHub Actions is the proposed CI/CD entry point.

## Trust and data boundaries

Request credentials are sensitive by default. Authorization headers, cookies,
credential headers, and user-designated secrets must be redacted before logs,
persistence, evidence artifacts, or MCP responses are produced. Evidence should
be immutable and traceable to an execution without retaining unneeded secrets.

The local agent is a separate trust boundary: it should bind locally by
default, require explicit authorization, and reveal the minimum information
needed by the hosted platform.

## Restore, limits and dispatch boundaries

Davian selected the MVP recovery exception: restoring an earlier point may
lose later changes and restore later-deleted projects/items. Show actual restore
timestamp(s) and this warning; deletion does not survive every restore. No
independently preserved anti-resurrection deletion journal. Normal access denial,
upload fencing, tombstones and orphan cleanup remain. Before reopening reconcile
Control, Protected and S3; missing evidence is unavailable, not fabricated.
Restored queued/running work and OUT must not automatically replay HTTP; require
deliberate new submissions/reruns under the proposed recovery gate.

Proposed size defaults are 64 KiB complete saved request configuration, 64 KiB
complete frozen execution configuration and independent 64 KiB resolved body;
2 MiB response wire-read and decompressed bounds, and 4 MiB complete encoded
evidence/API/local-upload caps. All configuration fields/serialization count.
No large request-body S3 subsystem in MVP. Proposed fast SQS notification after
durable commit improves cloud dispatch opportunity; durable OUT and scheduled
delivery recover failure/uncertainty without extra jobs or automatic HTTP replay.

## Current state

Only local process foundations and isolated, removable infrastructure-spike
definitions exist today; no Kurier AWS stage remains deployed. PostgreSQL is
available through Docker Compose, while an isolated Fargate spike proved
private RDS PostgreSQL connectivity, secure credential delivery, TLS, and a
versioned migration. Product services are not connected to PostgreSQL yet.
Queueing, evidence storage, authentication, and redaction remain future work.
