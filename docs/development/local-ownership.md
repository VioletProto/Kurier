# Local users/projects ownership slice

This is a Go local HTTP adapter and actual DynamoDB Local subset, not a deployed
Lambda/SST stage or full database migration. The [accepted contract](../architecture/users-projects-contract.md)
defines the seven routes. No executor, workflow, protected-secret subsystem,
S3 evidence or MCP is implemented by this task. The subsequent
[Cognito browser/local projects continuation](cognito-local-projects.md) adds
React auth/project UI and local CORS; its guide records cloud/email setup status.

## Setup

Require Go 1.26+, Node 22+ for repository checks, and Docker Compose. The separate
compose file pins the tested official DynamoDB Local image digest and binds
only 127.0.0.1:8000. It is **in memory**: stopping the container loses its local
data. The historical PostgreSQL compose file/spikes are unchanged and not needed.

From the repository root:

```sh
docker compose -f compose.dynamodb.yml up -d
export KURIER_DYNAMODB_ENDPOINT=http://127.0.0.1:8000
export KURIER_CONTROL_TABLE=kurier-local-control
export KURIER_STAGE=local
cd services/api
go run . -init-local
```

If the container is still starting, retry the initialization command after it
is ready. Init is local-only and idempotent; it never reactivates an existing
recovering stage. It creates Control PK/SK and the KEYS_ONLY GSI1 subset plus a
stage/generation marker. It does not create the full model or touch AWS. Local
SDK credentials are fixed nonsecret placeholders; AWS credential discovery,
HTTP proxies and redirects are disabled. Endpoints must be explicit HTTP
loopback URLs with a port; AWS/nonloopback URLs are rejected.

To start the actual API, provide an **existing** Cognito pool's issuer/client
configuration and a private random cursor signing key:

```sh
export KURIER_COGNITO_ISSUER=https://cognito-idp.us-east-2.amazonaws.com/EXISTING_POOL_ID
export KURIER_COGNITO_CLIENT_ID=EXISTING_PUBLIC_CLIENT_ID
export KURIER_CURSOR_KEY_BASE64="$(openssl rand -base64 32)"
go run .
```

Use an actual Cognito user-pool issuer, not the placeholders above. This does not
provision Cognito or request credentials automatically. Retain the cursor key
securely across restarts if outstanding cursors should remain valid; do not
commit it. No fixture issuer/client or signing key is a runtime option.
Missing/invalid authentication configuration or cursor key refuses startup;
there is no fallback authentication mode. The API binds 127.0.0.1:8080 by default
(`PORT` changes the port). `/healthz` retains `{"service":"api","status":"ok"}`;
it reports process liveness, not authentication/database readiness.

Application requests require a genuine `Authorization: Bearer <access token>`.
Do not put tokens in committed fixtures, shell commands/history, URLs or logs.
Obtain one using the existing pool's approved client flow; no pool/client/email
configuration was performed here. Without an existing pool, use the verified
fixture integration harness below, not an interactive authentication bypass.

## Runnable cleanup

After an owned DELETE returns 202, run locally from services/api:

```sh
go run . -cleanup-project PROJECT_ID_FROM_DELETE
```

This is an operator-only local maintenance command, not an unauthenticated API
endpoint. It needs the loopback database/table/stage settings above, not a JWT or
fixture signing configuration. It prints only safe deletionOperation metadata.
Repeat it after interruption; accepted ->deleting is separately durable and
completion/work removal is atomic. Repeating a completed cleanup is safe.

Unexpected children, missing work, generation/version conflicts or an unavailable
database cause a nonzero exit and keep the deletion pending. The command never
silently deletes unknown data. It may complete only the isolated empty-project
subset with no Protected/S3/child writers. Direct admin writes must obey the gate
protocol if racing cleanup; arbitrary out-of-protocol writes are not protected.
No AWS endpoint or credentials can be selected by these maintenance commands.

## Reproducible verification

From the repository root, with the container running:

```sh
npm ci
npm run verify
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
docker compose -f compose.dynamodb.yml config --quiet
git diff --check
```

Equivalent Go commands from services/api:

```sh
go test ./...
go vet ./...
go build -o /tmp/kurier-local-api .
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 go test -race -tags=integration -count=1 ./...
```

The integration tag **fails**, rather than silently skipping, if the endpoint
is missing/unreachable/nonloopback. It creates uniquely named test tables in
the actual official DynamoDB Local process. The local tables remain until the
in-memory container stops; no application table is dropped. Tests use bounded
startup readiness and invoke the cleanup CLI itself. CI adds a separate local
integration job on pull requests; its container shutdown is disposable.

Ephemeral RSA fixture keys, TLS JWKS trust roots and fixture issuer/client
configuration exist only in `_test.go`. Fixture tokens traverse the actual
RS256 verifier before resolving application users. They prove verification
against a trusted test issuer, **not authentication by Cognito**. Runtime accepts
only configured Cognito-shaped issuers and has no user-ID header, unsigned-token
mode, token mint route, development bypass flag or fixture trust option.

Checks cover real Local transactions/conditions and their failure boundaries:

- Seven routes, stable envelopes/ETags, validation, preconditions and health.
- Verified token rejection cases and bounded JWKS refresh/cache expiry.
- Concurrent lazy identity provisioning yields one mapping/user pair.
- Owner denial, disabled users, transaction rollback and one concurrent winner.
- Delete/rename races, immediate access denial, stable repeat DELETE and minimal
  tombstones, including initiating version zero.
- Interrupted cleanup, preserved unknown children, gate-respecting racing write,
  command-line resumption and repeat completion.
- Committed-but-lost creation acknowledgment: no automatic second creation;
  deletion acceptance is read back without another operation.
- Signed cursor scope/tampering/expiry/generation/high-water and newest-first
  pagination; bounded filtered empty pages. Stale candidates are explicitly
  seeded, not a claim about real AWS GSI propagation.

## Remaining validation boundaries

Local cannot establish cloud Cognito authentication/client setup/email delivery,
IAM, AWS throttling/capacity/transaction contention, GSI propagation timing,
PITR/restore, hosted JWT-authorizer integration, Lambda cold starts/networking,
SST resource composition or costs. The server implements token checks but does
not claim cloud authentication tested. Offline JWT revocation retains the
accepted token-lifetime lag; application disabled state is checked per request
and guarded during writes. JWKS uses HTTPS, RS256 RSA keys, configured issuer/
client, token_use=access, subject, required exp/iat, 60-second clock skew,
five-minute cache, 30-second refresh cooldown, two-second HTTP timeout and bounded
64 KiB/64-key responses. ID tokens, unknown algorithms and redirects are rejected.

No rate limiter, protected values, object storage, full project cascade or backup
recovery is implemented. Frontend/CORS integration and the development Cognito
stage are covered by the subsequent continuation guide.
This local adapter refuses nonloopback DynamoDB endpoints and stages without the
empty-project marker; production/cloud support needs a separately authorized
adapter, IAM and service-level validation. Future child writers require replacing
the empty-project cleanup path with reviewed full cross-store coordination.

To stop the disposable local database:

```sh
docker compose -f compose.dynamodb.yml down
```

## Checks actually performed for this task

The pinned Compose startup and local initialization command were executed.
`npm run verify` passed Prettier/gofmt checks, ESLint, the existing frontend test,
TypeScript/Vite build, and Go unit tests/vet in all five existing modules.
The API also built with `go build -o /tmp/kurier-local-api .`. The documented
race-enabled integration command passed against actual DynamoDB Local; the
final suite was repeated three times. Cleanup CLI success/repeat and unexpected-
child refusal were exercised by integration tests. Compose validation, relative
documentation links/fences and `git diff --check` passed.

These are local validation results, not AWS/Cognito/IAM/GSI propagation or full
cross-store deletion tests. No deployment, SST preview, migration against AWS,
cloud resource mutation or Cognito provisioning was performed.
