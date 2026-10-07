# Kurier

Kurier is an agent-ready API testing and debugging platform. Its core goal is
to produce immutable, sanitized API-execution evidence that developers and
coding agents can inspect safely.

This repository contains the first local users/projects DynamoDB ownership
slice and minimal foundations for later services. It is not a deployed product.

## Repository layout

- `apps/web` — React and TypeScript web application
- `services/api` — Go HTTP API
- `services/worker` — Go request-execution worker process
- `services/local-agent` — Go local execution agent and future MCP server
- `infra` — isolated infrastructure spikes and future infrastructure
- `docs` — architecture, decisions, spikes, and planning
- `scripts` — future repository-level development scripts

## Prerequisites

- Node.js 22 or newer and npm
- Go 1.26 or newer
- Docker with Docker Compose

## Setup and development

Follow the [Cognito browser/local projects guide](docs/development/cognito-local-projects.md)
for development authentication setup, startup, email participation and current
validation boundaries. Only authentication is eligible for AWS deployment;
the Go API and DynamoDB stay local.

After the authorized `dev-auth` deployment:

```sh
npm ci
docker compose -f compose.dynamodb.yml up -d
npm run dev:api -- -init-local
# In separate terminals:
npm run dev:api
npm run dev:auth
```

The historical PostgreSQL foundation/spikes remain separate. The
[local ownership guide](docs/development/local-ownership.md) describes manual
API configuration, strict JWT verification and operator-only cleanup.

The API defaults to `http://localhost:8080`, the worker health server to
`http://localhost:8081`, and the local agent to `http://localhost:8082`.
Each exposes `GET /healthz`.

## Verification

```sh
npm run format
npm run lint
npm test
npm run build
npm run verify:go
docker compose config --quiet
```

`npm run verify` runs all formatting checks, linting, tests, builds, Go tests,
and Go vet checks in one command.

For the actual DynamoDB Local integration suite:

```sh
docker compose -f compose.dynamodb.yml up -d
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
```

The integration tag fails when the local endpoint is absent; it is not part of
ordinary unit-only `npm run verify`. CI includes a separate integration job.

## SST viability configuration

The repository contains isolated, non-production SST configurations retained
from the completed compute/frontend and database viability experiments. The
configuration accepts only the temporary `viability` and `db-viability`
stages:

```sh
npm run sst:install
npm run sst:diff
npm run sst:deploy
npm run sst:state:list
npm run sst:remove
```

The database follow-up uses the corresponding `:db` commands:

```sh
npm run sst:install:db
npm run sst:diff:db
npm run sst:deploy:db
npm run sst:state:list:db
npm run sst:remove:db
```

These commands can create AWS resources and charges. Review
[`docs/spikes/sst-viability.md`](docs/spikes/sst-viability.md), authenticate
with a least-privilege short-lived AWS identity, inspect `npm run sst:diff`,
and obtain approval before deployment. The completed spikes support SST as
Kurier's selected infrastructure framework; the retained definitions remain
experimental and are not a production deployment.

The PostgreSQL credentials in `.env.example` and `compose.yml` are explicitly
for local development only. Never store real credentials in the repository.

## Architecture

See [the architecture overview](docs/architecture/README.md) and
[the SST viability spike](docs/spikes/sst-viability.md).
[ADR 0002](docs/decisions/0002-serverless-persistence-topology.md) records the
accepted serverless MVP baseline (`1fba942`), not implemented/deployed behavior.
The [accepted users/projects contract](docs/architecture/users-projects-contract.md)
records the subsequently authorized local subset, not hosted validation.
See the [acceptance and synchronization record](docs/architecture/w1-acceptance-sync.md)
for external-document updates and remaining proposed interface choices.
