# Infrastructure

This directory is reserved for infrastructure experiments and the future
production infrastructure definition.

SST 4 with TypeScript is selected in
[accepted ADR 0001](../docs/decisions/0001-sst-infrastructure-candidate.md).
[Accepted ADR 0002](../docs/decisions/0002-serverless-persistence-topology.md)
defines Go Lambda + HTTP API Gateway + SQS + DynamoDB + private S3 as the MVP
topology. Acceptance does not authorize deployment or AWS mutations.

The isolated Fargate/RDS services under `infra/spikes` and root SST configuration
are historical experiments, not the serverless product definition. Preserve
their [spike evidence](../docs/spikes/sst-viability.md); do not repeat the spikes
or interpret successful experiments as runtime validation of the accepted model.

The separately authorized `dev-auth` stage is the development authentication
subset in [development-auth.ts](development-auth.ts). It creates a user pool and
public browser client only; API/database/frontend remain local. The
[development guide](../docs/development/cognito-local-projects.md) records the
required session decision, default development sender and deployment checks.

The authorized `dev-api` stage in [development-api.ts](development-api.ts) adds
the retained on-demand Control table/indexes, ARM64 Go API/HTTP API and scheduled
empty-project maintenance. It reads the existing development Cognito pool; the
frontend remains local. See [deployment, cost and validation](../docs/development/aws-users-projects.md).
