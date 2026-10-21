export async function run(c) {
  const {
    ledger: l,
    privateState: p,
    check,
    aws,
    o,
    save,
    randomBytes,
    token,
    call,
    note,
  } = c;
  check(
    l.preflightVerified && !l.users.length && !l.projects.length,
    "Single disposable owner/project; no replay",
  );
  const name = "kurier-assertions-hl-" + l.tag + "@example.invalid";
  const u = { name, created: false };
  l.users.push(u);
  save();
  aws(["cognito-idp", "admin-create-user"], {
    UserPoolId: o.cognitoUserPoolId,
    Username: name,
    MessageAction: "SUPPRESS",
    UserAttributes: [
      { Name: "email", Value: name },
      { Name: "email_verified", Value: "true" },
    ],
  });
  u.created = true;
  save();
  const password = "Kurier1!" + randomBytes(24).toString("base64url");
  p.users.push({ name, password });
  aws(["cognito-idp", "admin-set-user-password"], {
    UserPoolId: o.cognitoUserPoolId,
    Username: name,
    Password: password,
    Permanent: true,
  });
  await token(0);
  const me = await call("GET", "/api/v1/users/me");
  check(me.data.user?.userId, "Owner resolution missing");
  u.owner = me.data.user.userId;
  save();
  const result = await call(
    "POST",
    "/api/v1/projects",
    { name: "Assertions H-L " + l.tag },
    201,
  );
  check(result.data.project?.projectId, "Project creation uncertain");
  l.projects.push({
    id: result.data.project.projectId,
    owner: 0,
    revision: result.data.project.version,
  });
  save();
  note("single-disposable-owner-project-created", {
    owner: u.owner,
    projectId: l.projects[0].id,
    cumulativeIdentities: 7,
    cumulativeProjects: 7,
  });
}
