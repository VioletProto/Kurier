// Public Price List catalog only; no resource mutation or billing/account data.
import { spawnSync } from "node:child_process";
const chosen = {
  AmazonDynamoDB:
    /ReadRequestUnits|WriteRequestUnits|TimedStorage-ByteHrs|PITR|BackupUsage|TimedBackupStorage/,
  AWSLambda: /Lambda-GB-Second-ARM$|Lambda-Requests$|Request$/,
  AmazonApiGateway:
    /ApiGatewayHttpApi$|ApiGatewayHttpRequest$|ApiGatewayHttpRequest-Tier1$/,
  AmazonS3: /Requests-Tier2$|TimedStorage-ByteHrs$|DataTransfer-Out-Bytes$/,
};
const rows = [];
for (const [service, pattern] of Object.entries(chosen)) {
  let token;
  for (let page = 0; page < 50; page++) {
    const args = [
      "pricing",
      "get-products",
      "--service-code",
      service,
      "--filters",
      "Type=TERM_MATCH,Field=regionCode,Value=us-east-2",
      "--max-results",
      "100",
      "--region",
      "us-east-1",
      "--output",
      "json",
      "--no-paginate",
    ];
    if (token) args.push("--next-token", token);
    const r = spawnSync("aws", args, {
      encoding: "utf8",
      maxBuffer: 16 * 1024 * 1024,
    });
    if (r.status !== 0) throw Error(`Price lookup unavailable: ${service}`);
    const data = JSON.parse(r.stdout);
    for (const raw of data.PriceList) {
      const p = JSON.parse(raw);
      const a = p.product.attributes;
      if (
        !pattern.test(a.usagetype ?? "") ||
        /IA-|Vector|Repl/.test(a.usagetype ?? "")
      )
        continue;
      for (const term of Object.values(p.terms.OnDemand ?? {}))
        for (const d of Object.values(term.priceDimensions ?? {})) {
          rows.push({
            service,
            usage: a.usagetype,
            operation: a.operation,
            storageClass: a.storageClass ?? null,
            unit: d.unit,
            begin: d.beginRange,
            end: d.endRange,
            usd: d.pricePerUnit.USD,
            description: d.description,
            effectiveDate: term.effectiveDate,
            publicationDate: p.publicationDate,
          });
        }
    }
    token = data.NextToken;
    if (!token) break;
    if (page === 49) throw Error("Price catalog pagination incomplete");
  }
}
console.log(
  JSON.stringify(
    {
      checkedAt: new Date().toISOString(),
      region: "us-east-2",
      source: "AWS Price List GetProducts",
      rates: rows,
    },
    null,
    2,
  ),
);
