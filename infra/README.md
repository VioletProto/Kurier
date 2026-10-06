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
