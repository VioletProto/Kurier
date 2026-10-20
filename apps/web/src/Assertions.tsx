import type { Assertion, Evidence } from "./api";
import { validAssertion } from "./assertions";

export function AssertionEditor({
  checks,
  onChange,
}: {
  checks: Assertion[];
  onChange: (value: Assertion[]) => void;
}) {
  const update = (i: number, a: Assertion) =>
    onChange(checks.map((v, j) => (j === i ? a : v)));
  return (
    <section aria-label="Step assertions">
      <p>
        Use public expected values only. Assertions cannot store or compare
        secrets. All checks must pass. At most eight checks and one status check
        per step.
      </p>
      {checks.map((a, i) => (
        <fieldset key={a.assertionId}>
          <legend>Assertion {i + 1}</legend>
          <label>
            Assertion kind
            <select
              value={a.kind}
              onChange={(e) => {
                const kind = e.target.value;
                update(
                  i,
                  kind === "statusEquals"
                    ? { assertionId: a.assertionId, kind, expected: 200 }
                    : kind === "jsonExists"
                      ? { assertionId: a.assertionId, kind, path: "$" }
                      : {
                          assertionId: a.assertionId,
                          kind: "jsonEquals",
                          path: "$",
                          expected: "",
                        },
                );
              }}
            >
              <option
                value="statusEquals"
                disabled={checks.some(
                  (v, j) => j !== i && v.kind === "statusEquals",
                )}
              >
                Status equals
              </option>
              <option value="jsonExists">JSON exists</option>
              <option value="jsonEquals">JSON equals</option>
            </select>
          </label>
          {a.kind !== "statusEquals" && (
            <label>
              Assertion JSON path
              <input
                value={a.path}
                onChange={(e) => update(i, { ...a, path: e.target.value })}
              />
            </label>
          )}
          {a.kind === "statusEquals" && (
            <label>
              Expected status
              <input
                type="number"
                min="100"
                max="599"
                step="1"
                value={a.expected}
                onChange={(e) =>
                  update(i, { ...a, expected: Number(e.target.value) })
                }
              />
            </label>
          )}
          {a.kind === "jsonEquals" && (
            <>
              <label>
                Expected type
                <select
                  value={
                    a.expected === null
                      ? "null"
                      : typeof a.expected === "object"
                        ? "number"
                        : typeof a.expected
                  }
                  onChange={(e) =>
                    update(i, {
                      ...a,
                      expected:
                        e.target.value === "number"
                          ? { type: "number", decimal: "" }
                          : e.target.value === "null"
                            ? null
                            : e.target.value === "boolean"
                              ? true
                              : "",
                    })
                  }
                >
                  <option value="string">String</option>
                  <option value="number">Number (exact decimal)</option>
                  <option value="boolean">Boolean</option>
                  <option value="null">Null</option>
                </select>
              </label>
              {typeof a.expected === "object" && a.expected !== null && (
                <label>
                  Expected decimal
                  <input
                    type="text"
                    value={a.expected.decimal}
                    onChange={(e) =>
                      update(i, {
                        ...a,
                        expected: { type: "number", decimal: e.target.value },
                      })
                    }
                  />
                </label>
              )}
              {typeof a.expected === "string" && (
                <label>
                  Expected string
                  <input
                    value={a.expected}
                    onChange={(e) =>
                      update(i, { ...a, expected: e.target.value })
                    }
                  />
                </label>
              )}
              {typeof a.expected === "boolean" && (
                <label>
                  Expected boolean
                  <select
                    value={String(a.expected)}
                    onChange={(e) =>
                      update(i, { ...a, expected: e.target.value === "true" })
                    }
                  >
                    <option value="true">true</option>
                    <option value="false">false</option>
                  </select>
                </label>
              )}
            </>
          )}
          {!validAssertion(a) && (
            <p>Complete a valid bounded assertion before saving.</p>
          )}
          <button
            disabled={i === 0}
            onClick={() => {
              const next = [...checks];
              [next[i - 1], next[i]] = [next[i], next[i - 1]];
              onChange(next);
            }}
          >
            Move assertion up
          </button>
          <button
            disabled={i === checks.length - 1}
            onClick={() => {
              const next = [...checks];
              [next[i + 1], next[i]] = [next[i], next[i + 1]];
              onChange(next);
            }}
          >
            Move assertion down
          </button>
          <button onClick={() => onChange(checks.filter((_, j) => j !== i))}>
            Remove assertion
          </button>
        </fieldset>
      ))}
      <button
        disabled={checks.length >= 8}
        onClick={() =>
          onChange([
            ...checks,
            { assertionId: crypto.randomUUID(), kind: "jsonExists", path: "$" },
          ])
        }
      >
        Add assertion
      </button>
    </section>
  );
}

export function AssertionResults({ evidence }: { evidence: Evidence }) {
  if (evidence.schemaVersion === 1 || !evidence.assertions)
    return <p>Assertion results unavailable.</p>;
  if (evidence.assertions.state === "not_configured")
    return <p>No assertions configured.</p>;
  return (
    <section aria-label="Assertion results">
      <h4>Assertion results</h4>
      <ol>
        {evidence.assertions.results.map((r) => (
          <li key={r.assertionId}>
            {r.kind}: {r.outcome.replaceAll("_", " ")} ({r.reason}); check{" "}
            {r.assertionId}
          </li>
        ))}
      </ol>
    </section>
  );
}
