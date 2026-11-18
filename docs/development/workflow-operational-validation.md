# Workflow operational validation

The [Trello card](https://trello.com/c/1QofXXKi) covers local workflow interruption and acknowledgment-loss validation. The [accepted workflow contract](../architecture/sequential-workflows-contract.md) defines the dispatch and recovery guarantees. The [hosted rollout record](sequential-workflows-publication-rollout.md) retains the earlier AWS observations.

## Local results

| Exercise                                                                     | Result                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Worker process killed after the controlled target received the first request | A separate process ran the workflow worker path against DynamoDB Local. The parent killed it while the loopback target withheld its response. The second step stayed pending and there was no published workflow output. After the fixture clock advanced past the dispatch lease, two deliveries to a fresh worker state left the run failed with `execution_outcome_unknown`, the second step skipped, and exactly one target request. |
| Lost acknowledgment after a committed workflow transaction                   | The injected fault left the successor durably scheduled. Processing it produced two total target calls for the two-step workflow.                                                                                                                                                                                                                                                                                                        |
| Lost result and uncertain evidence upload                                    | Injected final-transaction, lost-result, and lost-PUT acknowledgment cases kept the successor skipped and the target count at one. The committed assertion result stayed intact. Missing trustworthy results settled with unknown certainty.                                                                                                                                                                                             |
| Publication conflicts                                                        | Injected conflict exhaustion did not replay HTTP. Retry continued to respect project deletion, source revocation, and recovery-generation guards. A naturally occurring AWS transaction conflict is optional observational evidence.                                                                                                                                                                                                     |

The process termination and loopback request were real. The test used DynamoDB Local, KMS/S3/SQS doubles, and an advanced fixture clock for lease recovery. The acknowledgment failures and transaction conflicts were injected. No cloud fault or elapsed hosted retention was exercised.

Run the race-enabled API integration suite with the repository's pinned DynamoDB Local image bound to `127.0.0.1:8000`.

```sh
KURIER_DYNAMODB_TEST_ENDPOINT=http://127.0.0.1:8000 npm run test:ownership:integration
```

`npm run verify` and the full API race-enabled integration suite passed. The
focused workflow fault checks passed three consecutive runs. The local readback
showed no execution fixture tables after the suites exited. Removing the
disposable in-memory container cleared the other API fixture tables, and a
separate Docker readback confirmed the container was gone.

## Hosted follow-ups

- [Worker interruption and lost acknowledgments](https://trello.com/c/gmknhjVG)
- [Harness credential renewal](https://trello.com/c/AmXhBuKn)
- [Negative IAM checks](https://trello.com/c/SbFQwV2b)
- [Backup restoration and generation fencing](https://trello.com/c/Or39VkfO)
- [Elapsed retention and clock skew](https://trello.com/c/2owJdnRG)
- [Maximum-size workflow performance and cost](https://trello.com/c/smnNWida)

Each hosted card calls for evidence and cleanup readback for its own exercise. Cleanup throughput can be tracked separately if measured latency warrants a change.
