import { run as cleanup } from "./cleanup.mjs";
export async function run(c) {
  c.check(
    c.privateState.stopped && c.ledger.stoppedAt,
    "Recorded stop required",
  );
  await cleanup(c);
}
