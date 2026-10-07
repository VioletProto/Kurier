# Kurier repository context

Kurier is an agent-ready API testing and debugging platform. Its core value is
creating immutable, sanitized API-execution evidence that developers and
coding agents can inspect.

Security-sensitive request information must never be logged, persisted,
committed, or exposed to MCP without deliberate redaction. Authorization
headers, cookies, credential headers, and user-designated secrets must be
treated as sensitive.

The proposal and architecture are allowed to change when evidence supports a
better design. Prefer completing a reliable presentation-ready vertical slice
over maximizing feature count. Avoid implementing speculative abstractions or
stretch features before the MVP requires them.

Every completed task must be formatted, built, linted, and tested using the
relevant available commands before being declared complete.

# Repository workflow

- After fully completing and verifying a task, if this working directory is in
  a Git repository with a GitHub remote, commit only the changes made for that
  task and push them to GitHub.
- Use the user-specified branch when provided. Otherwise, continue an
  appropriate existing feature branch or create one for the work item.
  Never commit directly to `main`.
- Use a lightweight Conventional Commits format for commit subjects:
  `type: concise description` or, when useful,
  `type(scope): concise description`. Prefer lowercase types such as `feat`,
  `fix`, `refactor`, `build`, `chore`, `docs`, `test`, `style`, `perf`, `ci`,
  and `revert`.
- Preserve unrelated user changes in the working tree; do not include them in
  the task's commit.
- Never force-push, rewrite published history, or delete branches unless
  explicitly requested.
- If committing or pushing is unsafe, blocked, or would require a consequential
  choice, explain the issue instead of silently skipping submission.

# Solo-project workflow

- Davian is the sole developer; all Kurier cards and work belong to him.
  Explicit Trello assignment, membership, or Owner text is not required.
- For ticket work, read the target card, dependencies, checklist, and relevant
  board state. Follow an explicitly configured WIP limit; do not invent one.
- Read the current proposal and API contracts when relevant, and follow accepted
  repository ADRs. Refer to [accepted documentation](docs/architecture/README.md)
  and [ADRs](docs/decisions/) for architecture decisions.
- Keep work within the requested scope. Related documents and cards provide
  context, not authorization to implement additional features.
- Update card and checklist status to reflect verified work. Leave incomplete
  items open and read back updates to confirm they were applied.
- If a connector is unavailable, continue independent work and report the
  missing synchronization.

# AWS and validation

- Before AWS mutations, verify the intended account and a non-root identity.
  Use `us-east-2` unless instructed otherwise.
- Limit AWS changes to the resources and stage authorized by the current task.
  Inspect existing resources and planned changes, including SST bootstrap
  effects. Do not modify unrelated resources.
- Distinguish local and fixture tests from real AWS, Cognito, email, IAM, and
  runtime validation. Report material untested behavior honestly.
