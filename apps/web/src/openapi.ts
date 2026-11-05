// Editing server selection must not round numeric examples through JS Number.
export function choicesWithServer(source: string, server: string): string {
  const value: unknown = JSON.parse(source);
  if (!value || typeof value !== "object" || Array.isArray(value))
    throw Error("Operation inputs must be a JSON object.");
  const text = source.trim();
  const stringEnd = (start: number) => {
    let i = start + 1;
    while (i < text.length) {
      if (text[i] === "\\") i += 2;
      else if (text[i++] === '"') return i;
    }
    throw Error("Invalid operation inputs.");
  };
  let i = 1;
  while (i < text.length - 1) {
    while (/\s/.test(text[i] ?? "")) i++;
    if (text[i] === "}") break;
    const end = stringEnd(i);
    const key = JSON.parse(text.slice(i, end));
    i = end;
    while (/\s/.test(text[i] ?? "")) i++;
    i++;
    while (/\s/.test(text[i] ?? "")) i++;
    const start = i;
    let depth = 0;
    while (i < text.length) {
      if (text[i] === '"') i = stringEnd(i);
      else if (text[i] === "{" || text[i] === "[") {
        depth++;
        i++;
      } else if (text[i] === "}" || text[i] === "]") {
        if (depth === 0) break;
        depth--;
        i++;
      } else if (text[i] === "," && depth === 0) break;
      else i++;
    }
    if (key === "server")
      return text.slice(0, start) + JSON.stringify(server) + text.slice(i);
    if (text[i] === ",") i++;
  }
  return (
    '{"server":' +
    JSON.stringify(server) +
    (Object.keys(value).length ? "," : "") +
    text.slice(1)
  );
}

export type ContractResult = {
  importId: string;
  operationId: string;
  revision: number;
  result: {
    semanticsVersion: 1;
    status: "passed" | "failed" | "not_evaluated";
    checks: {
      kind: "status" | "headers" | "body";
      status: "passed" | "failed" | "not_evaluated";
      code?: string;
    }[];
  };
};
export function validContractResult(value: unknown): value is ContractResult {
  if (!value || typeof value !== "object") return false;
  const c = value as ContractResult;
  const state = (s: unknown) =>
    ["passed", "failed", "not_evaluated"].includes(String(s));
  return (
    typeof c.importId === "string" &&
    typeof c.operationId === "string" &&
    Number.isSafeInteger(c.revision) &&
    c.revision >= 0 &&
    c.result?.semanticsVersion === 1 &&
    state(c.result.status) &&
    Array.isArray(c.result.checks) &&
    c.result.checks.length === 3 &&
    c.result.checks.every((check, i) => {
      if (
        !check ||
        check.kind !== ["status", "headers", "body"][i] ||
        !state(check.status)
      )
        return false;
      if (check.status === "passed") return check.code === undefined;
      if (check.status === "not_evaluated")
        return [
          "sensitive_response",
          "not_dispatched",
          "evaluation_unavailable",
          "body_unavailable",
        ].includes(check.code ?? "");
      return (
        i === 0
          ? ["undocumented_status"]
          : i === 1
            ? ["missing_required_header"]
            : [
                "missing_required_property",
                "primitive_type_mismatch",
                "additional_property",
              ]
      ).includes(check.code ?? "");
    })
  );
}

const importCodes = new Set(
  "bundle_limit document_invalid example_invalid example_limit header_unsupported input_invalid inputs_required media_unsupported operation_invalid operation_limit operations_required parameter_duplicate parameter_invalid parameter_limit parameter_unsupported path_unsupported request_limit response_invalid serialization_unsupported server_limit server_unsupported source_unsafe dialect_unsupported feature_unsupported format_unsupported policy_invalid policy_limit reference_cycle reference_invalid reference_limit reference_sibling_unsupported reference_unsupported shape_invalid source_limit status_unsupported structure_limit schema_invalid schema_limit schema_unsupported key_invalid scalar_unsupported syntax_invalid yaml_alias_unsupported".split(
    " ",
  ),
);
export function importErrorMessage(code: string): string | undefined {
  return importCodes.has(code)
    ? `OpenAPI: ${code.replaceAll("_", " ")}. Check the supported format, limits and operation inputs.`
    : undefined;
}

export type OpenAPIImportSummary = {
  importId: string;
  projectId: string;
  revision: number;
  state: "creating" | "preparing" | "publishing" | "ready" | "deleting";
  operationCount: number;
  createdAt: string;
  updatedAt: string;
};
export type OpenAPICandidate = {
  operation: {
    operationId: string;
    method: string;
    path: string;
    servers: string[];
    parameters: {
      inputId: string;
      name: string;
      in: string;
      required: boolean;
      sensitive: boolean;
      schema: { type?: string };
    }[];
    requestBody?: { required: boolean };
  };
  choicesJSON: string;
  requestId: string;
  ready: boolean;
};
export type OpenAPIImport = OpenAPIImportSummary & {
  nextOperationOffset?: number | null;
  readyOperationCount?: number;
  diagnostics: { code: string }[];
  operations: OpenAPICandidate[];
};
