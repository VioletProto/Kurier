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
import {
  SessionBroker,
  SessionExpired,
  SessionUnavailable,
} from "./session-broker";
export {
  SessionExpired,
  SessionUnavailable,
  sessionChangedEvent,
} from "./session-broker";

const pool = import.meta.env.VITE_COGNITO_USER_POOL_ID;
const client = import.meta.env.VITE_COGNITO_CLIENT_ID;
export const authConfigured = Boolean(pool && client);
export const sessionBrokerEnabled =
  import.meta.env.VITE_KURIER_SESSION_BROKER === "true";
// Supported Amplify storage interface. Refresh material exists here only during
// the SRP-to-broker handoff; the pinned key suffix is covered by SDK adapter tests.
const transient = new Map<string, string>();
const handoffStorage = {
  async getItem(key: string) {
    return transient.get(key) ?? null;
  },
  async setItem(key: string, value: string) {
    transient.set(key, value);
  },
  async removeItem(key: string) {
    transient.delete(key);
  },
  async clear() {
    transient.clear();
  },
};
const broker = sessionBrokerEnabled ? new SessionBroker() : undefined;
if (authConfigured) {
  Amplify.configure({
    Auth: { Cognito: { userPoolId: pool, userPoolClientId: client } },
  });
  cognitoUserPoolsTokenProvider.setKeyValueStorage(
    broker ? handoffStorage : sharedInMemoryStorage,
  );
}

let generation = 0;
let active = false;
let refresh: Promise<string> | undefined;
export const auth = {
  restore: () => {
    if (!broker) return Promise.resolve(false);
    generation++;
    transient.clear();
    return broker.restore();
  },
  suspend() {
    if (broker) {
      generation++;
      transient.clear();
      broker.suspend();
    }
  },
  async login(email: string, password: string) {
    const loginGeneration = ++generation;
    if (broker?.hasAttempt(email)) {
      await broker.bootstrap(email);
      await broker.confirmCookie();
      transient.clear();
      return "done";
    }
    if (broker) {
      await broker.prepareLogin(email);
      transient.clear();
    }
    const result = await signIn({
      username: email,
      password,
      options: { authFlowType: "USER_SRP_AUTH" },
    });
    if (broker && loginGeneration !== generation) {
      transient.clear();
      throw new SessionExpired();
    }
    if (result.isSignedIn) {
      if (broker) {
        const session = await fetchAuthSession();
        if (loginGeneration !== generation) {
          transient.clear();
          throw new SessionExpired();
        }
        const access = session.tokens?.accessToken?.toString();
        const refreshValues = [...transient.entries()].filter(
          ([key]) =>
            key.startsWith(`CognitoIdentityServiceProvider.${client}.`) &&
            key.endsWith(".refreshToken"),
        );
        if (!access || refreshValues.length !== 1) throw new SessionExpired();
        try {
          await broker.bootstrap(email, access, refreshValues[0][1]);
          await broker.confirmCookie();
        } finally {
          transient.clear();
        }
      }
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
    if (broker) return broker.token(force);
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
    if (broker) {
      generation++;
      try {
        return await broker.logout();
      } finally {
        transient.clear();
        active = false;
        generation++;
      }
    }
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
  if (error instanceof SessionUnavailable || error instanceof SessionExpired)
    return error.message;
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
