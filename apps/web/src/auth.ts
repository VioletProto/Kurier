import { Amplify } from "aws-amplify";
import {
  fetchAuthSession,
  signIn,
  signUp,
  confirmSignUp,
  resendSignUpCode,
  resetPassword,
  confirmResetPassword,
  signOut,
} from "aws-amplify/auth";
import { cognitoUserPoolsTokenProvider } from "aws-amplify/auth/cognito";
import { sharedInMemoryStorage } from "aws-amplify/utils";

const pool = import.meta.env.VITE_COGNITO_USER_POOL_ID;
const client = import.meta.env.VITE_COGNITO_CLIENT_ID;
export const authConfigured = Boolean(pool && client);
if (authConfigured) {
  Amplify.configure({
    Auth: { Cognito: { userPoolId: pool, userPoolClientId: client } },
  });
  cognitoUserPoolsTokenProvider.setKeyValueStorage(sharedInMemoryStorage);
}

let generation = 0;
let active = false;
let refresh: Promise<string> | undefined;
export class SessionExpired extends Error {
  constructor() {
    super("Your session ended. Please sign in again.");
  }
}
export const auth = {
  async login(email: string, password: string) {
    const result = await signIn({
      username: email,
      password,
      options: { authFlowType: "USER_SRP_AUTH" },
    });
    if (result.isSignedIn) {
      active = true;
      generation++;
    }
    return result.isSignedIn
      ? "done"
      : result.nextStep.signInStep === "CONFIRM_SIGN_UP"
        ? "verify"
        : "unsupported";
  },
  signup: (email: string, password: string) =>
    signUp({
      username: email,
      password,
      options: { userAttributes: { email } },
    }),
  verify: (email: string, code: string) =>
    confirmSignUp({ username: email, confirmationCode: code }),
  resend: (email: string) => resendSignUpCode({ username: email }),
  reset: (email: string) => resetPassword({ username: email }),
  confirmReset: (email: string, code: string, password: string) =>
    confirmResetPassword({
      username: email,
      confirmationCode: code,
      newPassword: password,
    }),
  async token(force = false): Promise<string> {
    if (!active) throw new SessionExpired();
    if (refresh) return refresh;
    const current = generation;
    refresh = (async () => {
      const session = await fetchAuthSession({ forceRefresh: force }).catch(
        (error: unknown) => {
          const name = error instanceof Error ? error.name : "";
          if (
            [
              "NotAuthorizedException",
              "UserUnAuthenticatedException",
              "UserNotFoundException",
            ].includes(name)
          )
            throw new SessionExpired();
          throw new Error(
            "Cognito could not refresh this session. Check your connection and try again.",
          );
        },
      );
      if (!active || current !== generation || !session.tokens?.accessToken)
        throw new SessionExpired();
      return session.tokens.accessToken.toString();
    })().finally(() => {
      refresh = undefined;
    });
    return refresh;
  },
  async logout() {
    active = false;
    generation++;
    // Finish any SDK refresh before clearing storage, preventing late writes.
    await refresh?.catch(() => undefined);
    try {
      await signOut();
    } finally {
      sharedInMemoryStorage.clear();
    }
  },
};

export function authMessage(error: unknown): string {
  const name = error instanceof Error ? error.name : "";
  if (name === "CodeMismatchException" || name === "ExpiredCodeException")
    return "The code is invalid or expired. Request a new code and try again.";
  if (name === "InvalidPasswordException")
    return "Use at least 12 characters, including uppercase, lowercase, a number and a symbol.";
  if (name === "LimitExceededException" || name === "TooManyRequestsException")
    return "Too many attempts. Wait a few minutes before trying again.";
  if (name === "NetworkError")
    return "Cognito could not be reached. Check your connection and try again.";
  return "Unable to complete this step. Check your details, or use verification or password reset.";
}
