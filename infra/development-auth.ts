// Development authentication only. No hosted frontend, API or database.
export async function createDevelopmentAuth() {
  const expectedAccount = process.env.KURIER_AWS_ACCOUNT_ID;
  const tier = process.env.KURIER_AUTH_TIER;
  // The accepted Lite + rotation pair is unsupported by AWS. Require an
  // explicit reviewed choice instead of silently upgrading or disabling it.
  if (!expectedAccount || tier !== "ESSENTIALS") {
    throw new Error(
      "Set the intended AWS account and accepted KURIER_AUTH_TIER=ESSENTIALS.",
    );
  }
  const identity = await aws.getCallerIdentity({});
  if (
    identity.accountId !== expectedAccount ||
    identity.arn.endsWith(":root")
  ) {
    throw new Error(
      "Development auth requires the intended account and a non-root identity.",
    );
  }
  const pool = new sst.aws.CognitoUserPool("DevelopmentAuth", {
    usernames: ["email"],
    verify: {
      emailSubject: "Verify your Kurier account",
      emailMessage: "Your Kurier verification code is {####}.",
    },
    transform: {
      userPool(args) {
        args.userPoolTier = tier;
        args.mfaConfiguration = "OFF";
        args.accountRecoverySetting = {
          recoveryMechanisms: [{ name: "verified_email", priority: 1 }],
        };
        args.passwordPolicy = {
          minimumLength: 12,
          requireLowercase: true,
          requireUppercase: true,
          requireNumbers: true,
          requireSymbols: true,
          temporaryPasswordValidityDays: 1,
        };
        args.emailConfiguration = {
          emailSendingAccount: "COGNITO_DEFAULT",
        };
        args.userPoolAddOns = { advancedSecurityMode: "OFF" };
        args.deletionProtection = "ACTIVE";
        args.deviceConfiguration = undefined;
      },
    },
  });
  const client = pool.addClient("DevelopmentBrowser", {
    transform: {
      client(args) {
        args.generateSecret = false;
        args.explicitAuthFlows = ["ALLOW_USER_SRP_AUTH"];
        args.allowedOauthFlowsUserPoolClient = false;
        args.allowedOauthFlows = [];
        args.allowedOauthScopes = [];
        args.callbackUrls = [];
        args.accessTokenValidity = 15;
        args.idTokenValidity = 15;
        args.refreshTokenValidity = 1;
        args.tokenValidityUnits = {
          accessToken: "minutes",
          idToken: "minutes",
          refreshToken: "days",
        };
        args.refreshTokenRotation = {
          feature: "ENABLED",
          retryGracePeriodSeconds: 0,
        };
        args.enableTokenRevocation = true;
        args.preventUserExistenceErrors = "ENABLED";
        args.authSessionValidity = 3;
      },
    },
  });
  return {
    region: "us-east-2",
    stage: $app.stage,
    cognitoUserPoolId: pool.id,
    cognitoClientId: client.id,
    cognitoIssuer: pool.id.apply(
      (id) => `https://cognito-idp.us-east-2.amazonaws.com/${id}`,
    ),
    authTier: tier,
  };
}
