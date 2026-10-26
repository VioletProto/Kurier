import { useRef, useState } from "react";
import {
  api,
  ApiError,
  UncertainWrite,
  type EnvironmentSelection,
  type EnvironmentSummary,
  type EnvironmentWrite,
  type EnvironmentVariable,
  type PrimitiveValue,
  type ProjectEnvironment,
  type SecretMetadata,
} from "./api";
import { SessionExpired } from "./auth";
import { validAssertionDecimal } from "./assertions";

const initialValue = (type: PrimitiveValue["type"]): PrimitiveValue =>
  type === "number"
    ? { type, decimal: "0" }
    : type === "string"
      ? { type, value: "" }
      : type === "boolean"
        ? { type, value: false }
        : { type };
const draftOf = (env: ProjectEnvironment): EnvironmentWrite => ({
  name: env.name,
  variables: env.variables.map((v) =>
    v.sensitive
      ? {
          variableId: v.variableId,
          name: v.name,
          enabled: v.enabled,
          sensitive: true,
          valueType: v.valueType,
          secretWrite: { action: "preserve", secretRef: v.secretRef! },
        }
      : {
          variableId: v.variableId,
          name: v.name,
          enabled: v.enabled,
          sensitive: false,
          value: v.value,
        },
  ),
});
function clearSets(draft: EnvironmentWrite): EnvironmentWrite {
  return {
    ...draft,
    variables: draft.variables.map((v) =>
      v.secretWrite?.action === "set"
        ? {
            ...v,
            secretWrite: undefined,
          }
        : v,
    ),
  };
}
function variableValue(v: EnvironmentVariable): PrimitiveValue | undefined {
  return v.sensitive
    ? v.secretWrite?.action === "set"
      ? v.secretWrite.value
      : undefined
    : v.value;
}
function validDraft(draft: EnvironmentWrite): boolean {
  const names = new Set<string>();
  return (
    draft.name.trim().length > 0 &&
    draft.variables.length <= 32 &&
    draft.variables.filter((v) => v.sensitive).length <= 16 &&
    draft.variables.every((v) => {
      if (!/^[A-Za-z_][A-Za-z0-9_]{0,63}$/.test(v.name) || names.has(v.name))
        return false;
      names.add(v.name);
      const value = variableValue(v);
      if (!value)
        return (
          v.sensitive &&
          !!v.secretWrite &&
          v.secretWrite.action !== "set" &&
          !!v.secretWrite.secretRef?.secretId
        );
      if (value.type === "number" && !validAssertionDecimal(value.decimal))
        return false;
      const scalar =
        value.type === "number"
          ? value.decimal
          : value.type === "null"
            ? "null"
            : JSON.stringify(value.value);
      return (
        new TextEncoder().encode(scalar).length <= 2048 &&
        !(v.sensitive && value.type === "string" && !value.value)
      );
    })
  );
}
export function EnvironmentSelector({
  projectId,
  label,
  value,
  change,
  disabled,
  onExpired,
}: {
  projectId: string;
  label: string;
  value: EnvironmentSelection | null;
  change: (v: EnvironmentSelection | null) => void;
  disabled: boolean;
  onExpired: () => void;
}) {
  const [items, setItems] = useState<EnvironmentSummary[]>([]),
    [cursor, setCursor] = useState<string | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const load = async (more = false) => {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      const page = await api.listEnvironments(projectId, more ? cursor : null);
      setItems((old) =>
        more
          ? [
              ...old,
              ...page.items.filter(
                (x) => !old.some((v) => v.environmentId === x.environmentId),
              ),
            ]
          : page.items,
      );
      setCursor(page.nextCursor);
    } catch (e) {
      if (e instanceof SessionExpired) onExpired();
      else
        setError(
          e instanceof ApiError
            ? e.message
            : "Environment choices unavailable.",
        );
    } finally {
      setBusy(false);
    }
  };
  return (
    <fieldset disabled={disabled || busy}>
      <legend>{label}</legend>
      <select
        aria-label={label}
        value={value?.environmentId ?? ""}
        onChange={(e) => {
          const selected = items.find(
            (x) => x.environmentId === e.target.value,
          );
          change(
            selected
              ? {
                  environmentId: selected.environmentId,
                  revision: selected.revision,
                }
              : null,
          );
        }}
      >
        <option value="">No environment</option>
        {value &&
          !items.some((v) => v.environmentId === value.environmentId) && (
            <option value={value.environmentId}>
              Selected environment · revision {value.revision}
            </option>
          )}
        {items.map((v) => (
          <option key={v.environmentId} value={v.environmentId}>
            {v.name} · revision {v.revision}
          </option>
        ))}
      </select>
      <button type="button" onClick={() => void load()}>
        Refresh environment choices
      </button>
      {cursor && (
        <button type="button" onClick={() => void load(true)}>
          More environment choices
        </button>
      )}
      {error && <p role="alert">{error}</p>}
    </fieldset>
  );
}
export function Environments({
  projectId,
  disabled,
  onExpired,
  onGateChanged,
}: {
  projectId: string;
  disabled: boolean;
  onExpired: () => void;
  onGateChanged: () => void;
}) {
  const [opened, setOpened] = useState(false),
    [items, setItems] = useState<EnvironmentSummary[]>([]),
    [cursor, setCursor] = useState<string | null>(null),
    [selected, setSelected] = useState<ProjectEnvironment>(),
    [draft, setDraft] = useState<EnvironmentWrite>({ name: "", variables: [] }),
    [secrets, setSecrets] = useState<SecretMetadata[]>([]),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [uncertain, setUncertain] = useState(false),
    [deleteConfirm, setDeleteConfirm] = useState(false);
  const lock = useRef(false);
  const action = async (fn: () => Promise<void>) => {
    if (lock.current || disabled) return;
    lock.current = true;
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      if (e instanceof SessionExpired) onExpired();
      else {
        if (e instanceof UncertainWrite) setUncertain(true);
        setError(
          e instanceof ApiError || e instanceof UncertainWrite
            ? e.message
            : "Environment operation could not be completed.",
        );
      }
    } finally {
      lock.current = false;
      setBusy(false);
    }
  };
  const refresh = async (more = false) => {
    const page = await api.listEnvironments(projectId, more ? cursor : null);
    setItems((old) =>
      more
        ? [
            ...old,
            ...page.items.filter(
              (x) => !old.some((v) => v.environmentId === x.environmentId),
            ),
          ]
        : page.items,
    );
    setCursor(page.nextCursor);
    setUncertain(false);
  };
  const open = async (id: string) => {
    const env = await api.getEnvironment(projectId, id);
    setSelected(env);
    setDraft(draftOf(env));
    setUncertain(false);
    setDeleteConfirm(false);
  };
  const change = (
    i: number,
    update: (v: EnvironmentVariable) => EnvironmentVariable,
  ) =>
    setDraft((old) => ({
      ...old,
      variables: old.variables.map((v, n) => (n === i ? update(v) : v)),
    }));
  const save = async () => {
    if (!validDraft(draft)) {
      setDraft(clearSets(draft));
      setError(
        "Check unique variable names, types, exact decimal values and size limits. Protected drafts were cleared.",
      );
      return;
    }
    const pending = api.saveEnvironment(projectId, draft, selected);
    setDraft(clearSets(draft));
    const env = await pending;
    setSelected(env);
    setDraft(draftOf(env));
    setUncertain(false);
    onGateChanged();
    await refresh();
  };
  return (
    <section className="panel" aria-label="Project environments">
      <h3>Project environments</h3>
      <p>
        Public values are saved configuration. Protected values are write-only
        encrypted inputs and are never previewed. Numbers remain exact decimal
        text.
      </p>
      <button
        disabled={disabled || busy}
        onClick={() =>
          void action(async () => {
            setOpened(true);
            await refresh();
          })
        }
      >
        Open environments
      </button>
      {opened && (
        <>
          <button
            disabled={disabled || busy}
            onClick={() =>
              void action(async () => {
                await refresh();
                if (selected) await open(selected.environmentId);
              })
            }
          >
            Refresh environments
          </button>
          <button
            disabled={disabled || busy || uncertain}
            onClick={() => {
              setSelected(undefined);
              setDraft({ name: "", variables: [] });
              setDeleteConfirm(false);
            }}
          >
            New environment
          </button>
          <ul>
            {items.map((v) => (
              <li key={v.environmentId}>
                <button
                  disabled={disabled || busy}
                  onClick={() => void action(() => open(v.environmentId))}
                >
                  {v.name} · revision {v.revision}
                </button>
              </li>
            ))}
          </ul>
          {cursor && (
            <button
              disabled={disabled || busy}
              onClick={() => void action(() => refresh(true))}
            >
              Load more environments
            </button>
          )}
          {uncertain && (
            <p role="status">
              The write result is uncertain. Refresh and inspect existing
              environments before another write. Protected drafts have been
              cleared.
            </p>
          )}
          <form
            autoComplete="off"
            onInvalidCapture={() => setDraft(clearSets(draft))}
            onSubmit={(e) => {
              e.preventDefault();
              void action(save);
            }}
          >
            <fieldset disabled={disabled || busy || uncertain}>
              <legend>
                {selected
                  ? `Edit environment · revision ${selected.revision}`
                  : "Create environment"}
              </legend>
              <label>
                Environment name
                <input
                  value={draft.name}
                  autoComplete="off"
                  onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                  required
                  maxLength={100}
                />
              </label>
              {draft.variables.map((v, i) => {
                const value = variableValue(v),
                  type = v.valueType ?? v.value?.type ?? "string";
                return (
                  <fieldset key={v.variableId ?? i}>
                    <legend>Variable {i + 1}</legend>
                    <label>
                      Variable name {i + 1}
                      <input
                        value={v.name}
                        disabled={!!v.variableId}
                        onChange={(e) =>
                          change(i, (v) => ({ ...v, name: e.target.value }))
                        }
                        autoComplete="off"
                        required
                        pattern="[A-Za-z_][A-Za-z0-9_]{0,63}"
                      />
                    </label>
                    <label>
                      Variable type {i + 1}
                      <select
                        aria-label={`Variable type ${i + 1}`}
                        disabled={!!v.variableId}
                        value={type}
                        onChange={(e) =>
                          change(i, (v) =>
                            v.sensitive
                              ? {
                                  ...v,
                                  valueType: e.target
                                    .value as PrimitiveValue["type"],
                                  secretWrite: {
                                    action: "set",
                                    value: initialValue(
                                      e.target.value as PrimitiveValue["type"],
                                    ),
                                  },
                                }
                              : {
                                  ...v,
                                  value: initialValue(
                                    e.target.value as PrimitiveValue["type"],
                                  ),
                                },
                          )
                        }
                      >
                        {["string", "number", "boolean", "null"].map((t) => (
                          <option key={t}>{t}</option>
                        ))}
                      </select>
                    </label>
                    <label>
                      <input
                        type="checkbox"
                        checked={v.enabled}
                        onChange={(e) =>
                          change(i, (v) => ({
                            ...v,
                            enabled: e.target.checked,
                          }))
                        }
                      />
                      Enabled variable {i + 1}
                    </label>
                    <label>
                      <input
                        type="checkbox"
                        checked={v.sensitive}
                        disabled={!!v.variableId}
                        onChange={(e) =>
                          change(i, (v) =>
                            e.target.checked
                              ? {
                                  variableId: v.variableId,
                                  name: v.name,
                                  enabled: v.enabled,
                                  sensitive: true,
                                  valueType: type,
                                  secretWrite: {
                                    action: "set",
                                    value: initialValue(type),
                                  },
                                }
                              : {
                                  variableId: v.variableId,
                                  name: v.name,
                                  enabled: v.enabled,
                                  sensitive: false,
                                  value: initialValue(type),
                                },
                          )
                        }
                      />
                      Protected variable {i + 1}
                    </label>
                    {v.sensitive && (
                      <>
                        <label>
                          Protected action {i + 1}
                          <select
                            aria-label={`Protected action ${i + 1}`}
                            value={v.secretWrite?.action ?? ""}
                            onChange={(e) =>
                              change(i, (v) =>
                                e.target.value === ""
                                  ? { ...v, secretWrite: undefined }
                                  : e.target.value === "set"
                                    ? {
                                        ...v,
                                        secretWrite: {
                                          action: "set",
                                          value: initialValue(type),
                                        },
                                      }
                                    : {
                                        ...v,
                                        secretWrite: {
                                          action: e.target.value as
                                            "preserve" | "secretRef",
                                          secretRef: selected?.variables.find(
                                            (original) =>
                                              original.variableId ===
                                              v.variableId,
                                          )?.secretRef ?? { secretId: "" },
                                        },
                                      },
                              )
                            }
                          >
                            <option value="">Choose a protected action</option>
                            <option value="set">
                              Set a fresh protected value
                            </option>
                            {v.variableId && (
                              <option value="preserve">
                                Preserve exact stored secret
                              </option>
                            )}
                            <option value="secretRef">
                              Attach a compatible saved secret
                            </option>
                          </select>
                        </label>
                        {v.secretWrite?.action === "secretRef" && (
                          <>
                            <button
                              type="button"
                              onClick={() =>
                                void action(async () =>
                                  setSecrets(
                                    (await api.listSecrets(projectId)).items,
                                  ),
                                )
                              }
                            >
                              Load compatible saved secrets
                            </button>
                            <select
                              aria-label={`Saved secret ${i + 1}`}
                              value={v.secretWrite.secretRef.secretId}
                              onChange={(e) =>
                                change(i, (v) => ({
                                  ...v,
                                  secretWrite: {
                                    action: "secretRef",
                                    secretRef: { secretId: e.target.value },
                                  },
                                }))
                              }
                            >
                              <option value="">Choose a saved secret</option>
                              {secrets
                                .filter(
                                  (s) =>
                                    s.state === "active" &&
                                    (s.scalarType ??
                                      (s.valueKind === "string"
                                        ? "string"
                                        : "untyped")) === type,
                                )
                                .map((s) => (
                                  <option key={s.secretId} value={s.secretId}>
                                    {s.secretId} · {s.mask}
                                  </option>
                                ))}
                            </select>
                          </>
                        )}
                        {v.secretWrite?.action === "preserve" && (
                          <p>
                            Stored value masked. Changing it here requires a
                            fresh secret; shared secret PATCH updates future
                            admissions and reruns.
                          </p>
                        )}
                      </>
                    )}
                    {value && (
                      <>
                        {type === "boolean" ? (
                          <label>
                            Variable value {i + 1}
                            <select
                              value={String(
                                value.type === "boolean" && value.value,
                              )}
                              onChange={(e) =>
                                change(i, (v) => {
                                  const value: PrimitiveValue = {
                                    type: "boolean",
                                    value: e.target.value === "true",
                                  };
                                  return v.sensitive
                                    ? {
                                        ...v,
                                        secretWrite: { action: "set", value },
                                      }
                                    : { ...v, value };
                                })
                              }
                            >
                              <option value="false">false</option>
                              <option value="true">true</option>
                            </select>
                          </label>
                        ) : type === "null" ? (
                          <p>Present JSON null</p>
                        ) : (
                          <label>
                            Variable value {i + 1}
                            <input
                              type={v.sensitive ? "password" : "text"}
                              autoComplete="off"
                              spellCheck={false}
                              value={
                                value.type === "number"
                                  ? value.decimal
                                  : value.type === "string"
                                    ? value.value
                                    : ""
                              }
                              onChange={(e) =>
                                change(i, (v) => {
                                  const value: PrimitiveValue =
                                    type === "number"
                                      ? { type, decimal: e.target.value }
                                      : {
                                          type: "string",
                                          value: e.target.value,
                                        };
                                  return v.sensitive
                                    ? {
                                        ...v,
                                        secretWrite: { action: "set", value },
                                      }
                                    : { ...v, value };
                                })
                              }
                            />
                          </label>
                        )}
                      </>
                    )}
                    {v.variableId && (
                      <p>
                        Identity, name, type and protection are fixed. Remove
                        and add a new variable to change them.
                      </p>
                    )}
                    <button
                      type="button"
                      onClick={() =>
                        setDraft({
                          ...draft,
                          variables: draft.variables.filter((_, n) => n !== i),
                        })
                      }
                    >
                      Remove variable {i + 1}
                    </button>
                  </fieldset>
                );
              })}
              <button
                type="button"
                disabled={draft.variables.length >= 32}
                onClick={() =>
                  setDraft({
                    ...draft,
                    variables: [
                      ...draft.variables,
                      {
                        name: "",
                        enabled: true,
                        sensitive: false,
                        value: { type: "string", value: "" },
                      },
                    ],
                  })
                }
              >
                Add variable
              </button>
              <button type="submit">
                {selected ? "Save environment changes" : "Create environment"}
              </button>
              {selected && (
                <button type="button" onClick={() => setDeleteConfirm(true)}>
                  Delete environment
                </button>
              )}
              {selected && deleteConfirm && (
                <>
                  <p>
                    Delete this environment permanently? Frozen evidence keeps
                    its identity, but future dispatch and reruns requiring it
                    will be blocked. Shared secrets remain active until
                    separately revoked.
                  </p>
                  <button
                    type="button"
                    onClick={() =>
                      void action(async () => {
                        await api.deleteEnvironment(projectId, selected);
                        setSelected(undefined);
                        setDraft({ name: "", variables: [] });
                        setDeleteConfirm(false);
                        onGateChanged();
                        await refresh();
                      })
                    }
                  >
                    Confirm environment deletion
                  </button>
                </>
              )}
            </fieldset>
          </form>
        </>
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
