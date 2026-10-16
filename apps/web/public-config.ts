export const publicKeys = [
  "VITE_API_URL",
  "VITE_COGNITO_USER_POOL_ID",
  "VITE_COGNITO_CLIENT_ID",
  "VITE_KURIER_STAGE",
] as const;

export function hostedPublicConfig(env: Record<string, unknown>) {
  if (
    Object.keys(env).some(
      (key) =>
        key.startsWith("VITE_") &&
        !publicKeys.includes(key as (typeof publicKeys)[number]),
    ) ||
    env.VITE_KURIER_STAGE !== "dev-api" ||
    !/^https:\/\/[a-z0-9]+\.execute-api\.us-east-2\.amazonaws\.com$/.test(
      String(env.VITE_API_URL ?? ""),
    ) ||
    !/^us-east-2_[A-Za-z0-9]+$/.test(
      String(env.VITE_COGNITO_USER_POOL_ID ?? ""),
    ) ||
    !/^[a-z0-9]+$/.test(String(env.VITE_COGNITO_CLIENT_ID ?? ""))
  )
    throw new Error("Hosted public configuration is missing or invalid.");
  return Object.fromEntries(
    publicKeys.map((key) => [key, String(env[key])]),
  ) as Record<(typeof publicKeys)[number], string>;
}
