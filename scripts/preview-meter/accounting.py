"""Frozen inventory135 + preview30 + two uncertain refreshes =167, never a reset."""

import hashlib
import json

INVENTORY_SHA = "1e4e9f987fdf378ac2b6b968d65175a1a1a5d3d5b8085c8145d86c434b937171"
PREVIEW_SHA = "e65635f9b3e21f406cb70dcd0ba7ae71f952e203e54d30f6474da591abe525eb"
AUTH_SHA = "5316b2ec89fd7ff682f175a6cd14008a840fe2438814c368ab15a05023b997d7"
CLAIM_SHA = "87e39ae4b58c5942d951a7c12b5f5534e23c6f8fc64a8fc16db591921f5bd108"
PROJECTION_SHA = "a70542adbdab540682f958d826289ca70191b97f0e00cdf28bc0a71d07f42c99"
AUTH4_SHA = "0bb82bf9b14cd14a95861424bf3103ebb13bdf8218c08cf5444965837a72de69"
CLAIM4_SHA = "68f93672420d6ffff27c74417351516e339687e1ea4384e6a646bd63a619039a"
PROJECTION4_SHA = "e8d05b3663a499f8225bc3d68d1dd74d4ea4b4fed34d5418ccb1950a684b33b1"
PRIOR = 167
MAXIMUM = 500
FROZEN_FILES = {
    "docs/development/project-environments-inventory-attempts.jsonl": INVENTORY_SHA,
    "docs/development/project-environments-preview-attempts.jsonl": PREVIEW_SHA,
    "docs/development/project-environments-refresh-preview-auth-projection.jsonl": PROJECTION_SHA,
    ".build/project-environments-preview/review-3/authentication.jsonl": AUTH_SHA,
    ".build/project-environments-preview/review-3/window.json": "19c362f32d5ce9e0ea4c8ad67c9f76e8ee03130f58c3e257aadbcdf1aa4c8f39",
    ".build/project-environments-preview/review-3/cumulative-usage.json": "346fa66374bac7ac87fd48ef250e4eb438d8d3ad725ca23876f9a7cacf684671",
    ".build/project-environments-preview/window-from-165.json": CLAIM_SHA,
    "docs/development/project-environments-refresh-preview-review-4-auth-projection.jsonl": PROJECTION4_SHA,
    ".build/project-environments-preview/review-4/authentication.jsonl": AUTH4_SHA,
    ".build/project-environments-preview/review-4/window.json": "538921f5c442ae7b2f3492267c62c8fe55bf1cd3f060e34d486923cde60b911e",
    ".build/project-environments-preview/review-4/cumulative-usage.json": "1cca6aaeef18233ab7db538f4560684375167c731a8182b04756c7e2c5595765",
    ".build/project-environments-preview/window-from-166.json": CLAIM4_SHA,
}


def baseline(root):
    inventory = root / "docs/development/project-environments-inventory-attempts.jsonl"
    preview = root / "docs/development/project-environments-preview-attempts.jsonl"
    if hashlib.sha256(inventory.read_bytes()).hexdigest() != INVENTORY_SHA:
        raise ValueError("inventory-ledger-hash")
    if hashlib.sha256(preview.read_bytes()).hexdigest() != PREVIEW_SHA:
        raise ValueError("preview-ledger-hash")
    for name, expected in FROZEN_FILES.items():
        if hashlib.sha256((root / name).read_bytes()).hexdigest() != expected:
            raise ValueError("frozen-accounting-proof-hash")
    old = [json.loads(x) for x in inventory.read_text().splitlines()]
    new = [json.loads(x) for x in preview.read_text().splitlines()]
    old_calls = [r["count"] for r in old if r["event"] == "attempt"]
    new_calls = [r["count"] for r in new if r["event"] == "attempt"]
    if old_calls != list(range(1, 136)) or new_calls != list(range(136, 166)):
        raise ValueError("ledger-sequence")
    if (
        old[-1]["event"] != "stop"
        or new[-1]["event"] != "stop"
        or new[-1]["count"] != 165
    ):
        raise ValueError("stopped-ledger-required")
    failed = root / ".build/project-environments-preview/review-3"
    auth = [
        json.loads(x)
        for x in (failed / "authentication.jsonl").read_text().splitlines()
    ]
    if (
        [r["event"] for r in auth]
        != ["created", "shared-window", "attempt", "outcome", "stop"]
        or auth[0]["prior"] != 165
        or auth[2]["count"] != 166
        or (auth[2]["service"], auth[2]["operation"]) != ("signin", "CreateOAuth2Token")
        or auth[3]["attempt"] != 166
        or auth[3]["status"] is not None
        or auth[3]["transportError"] is not True
        or auth[4]["count"] != 166
        or auth[4]["reason"] != "transport-failure"
        or (failed / "handoff.json").exists()
        or (failed / "provider-claim.json").exists()
    ):
        raise ValueError("failed-authentication-proof")
    failed4 = root / ".build/project-environments-preview/review-4"
    auth4 = [
        json.loads(x)
        for x in (failed4 / "authentication.jsonl").read_text().splitlines()
    ]
    if (
        [r["event"] for r in auth4]
        != [
            "created",
            "shared-window",
            "attempt",
            "authentication-transport-error",
            "outcome",
            "stop",
        ]
        or auth4[0]["prior"] != 166
        or auth4[2]["count"] != PRIOR
        or (auth4[2]["service"], auth4[2]["operation"])
        != ("signin", "CreateOAuth2Token")
        or auth4[3]["attempt"] != PRIOR
        or auth4[3]["exceptionCategory"] != "connect-timeout"
        or auth4[3]["failurePhase"] != "connect"
        or auth4[3]["errorCode"] is not None
        or auth4[3]["errorCodeDomain"] is not None
        or auth4[4]["attempt"] != PRIOR
        or auth4[4]["status"] is not None
        or auth4[4]["transportError"] is not True
        or auth4[5]["count"] != PRIOR
        or auth4[5]["reason"] != "transport-failure"
        or (failed4 / "handoff.json").exists()
        or (failed4 / "provider-claim.json").exists()
    ):
        raise ValueError("second-failed-authentication-proof")
    return {
        "prior": PRIOR,
        "maximum": MAXIMUM,
        "inventoryLedgerSha256": INVENTORY_SHA,
        "stoppedPreviewLedgerSha256": PREVIEW_SHA,
        "failedAuthenticationLedgerSha256": AUTH_SHA,
        "failedCampaignClaimSha256": CLAIM_SHA,
        "failedAuthenticationProjectionSha256": PROJECTION_SHA,
        "secondFailedAuthenticationLedgerSha256": AUTH4_SHA,
        "secondFailedCampaignClaimSha256": CLAIM4_SHA,
        "secondFailedAuthenticationProjectionSha256": PROJECTION4_SHA,
    }


# Preserve the historical 167-based diagnostic executor and baseline above.
# Authentication/preview alone now consume the terminal diagnostic's charge.
BASELINE_168_PRIOR = 168
BASELINE_168_FILES = {
    **FROZEN_FILES,
    ".build/project-environments-network-diagnostics/review-1/attempts.jsonl": "ad009e85a025ebbbd6b8b9398b56e4ea799d3d705e3b85d393ba0f55db9da9e9",
    ".build/project-environments-network-diagnostics/review-1/result.json": "d0bd6f4c37cb92dbe18b33a995e4975688a24a04505b2d34881ac42cc3699432",
    ".build/project-environments-network-diagnostics/review-1/context.json": "5b442af2a0ea0a552e93b476c6a22e6234ba2cce8e1ce5b032e13fe55d50f4fe",
    ".build/project-environments-network-diagnostics/review-1/addresses.json": "b45ee55d1ba9a6781ff8a557dfb541fb388c7a3165bad4745f58c89cb6ed5874",
    ".build/project-environments-network-diagnostics/review-1/route-4.json": "f7b7e49d6681f43ef44abebc61aa8930d7e112d95d4d2a0b04eaf9397f91e9a0",
    ".build/project-environments-preview/window-from-167.json": "9e27595362462d7c24741b98dcc2274c152af3cbec2093b5c1b9d822e276181b",
    ".build/project-environments-network-probe-offline/gate.json": "47eb5efd7a343640c49f69865487e7becc961b64195929719b92a637bfadf922",
    "docs/development/project-environments-network-probe-ledger.jsonl": "2e3beb072185a8636635181f10a6efcb0733961369df0b23205fe1e9a5a470aa",
    "docs/development/project-environments-network-probe-result.json": "80294d852d9c4d02dde6edf23854e14cdda4c2bcc6534bbbd325ddf1f580c262",
}


def baseline_168(root):
    previous = baseline(root)
    for name, expected in BASELINE_168_FILES.items():
        if hashlib.sha256((root / name).read_bytes()).hexdigest() != expected:
            raise ValueError("current-accounting-proof-hash")
    rows = [
        json.loads(line)
        for line in (
            root / "docs/development/project-environments-network-probe-ledger.jsonl"
        )
        .read_text()
        .splitlines()
    ]
    attempts = [r for r in rows if r["event"] == "attempt"]
    if (
        previous["prior"] != 167
        or rows[0]["prior"] != 167
        or [(r["kind"], r["ordinal"]) for r in attempts]
        != [("dns", 1), ("dns", 2), ("tcp", 1)]
        or attempts[-1]["cumulative"] != BASELINE_168_PRIOR
        or rows[-1]["event"] != "stop"
        or rows[-1]["reason"] != "tcp-failed"
        or rows[-1]["counters"] != {"dns": 2, "tcp": 1, "tls": 0}
        or rows[-1]["cumulative"] != BASELINE_168_PRIOR
    ):
        raise ValueError("terminal-diagnostic-accounting")
    return {
        **previous,
        "prior": BASELINE_168_PRIOR,
        "diagnosticLedgerSha256": BASELINE_168_FILES[
            ".build/project-environments-network-diagnostics/review-1/attempts.jsonl"
        ],
        "diagnosticCampaignClaimSha256": BASELINE_168_FILES[
            ".build/project-environments-preview/window-from-167.json"
        ],
    }


# The consumed review-5 campaign charged two authentication/verification and
# thirty provider attempts. New tooling cannot reset to its historical parent.
BASELINE_200_PRIOR = 200
BASELINE_200_FILES = {
    **BASELINE_168_FILES,
    ".build/project-environments-preview/review-5/authentication.jsonl": "501661d3f821573433eb8346f8743aa183ff71306b875bb2b69a81c0ba28e4d7",
    ".build/project-environments-preview/review-5/window.json": "58ad715a37148bbf4524cdcdd483ba1eb6054963c9685686c0ac8c12f3154e49",
    ".build/project-environments-preview/review-5/handoff.json": "cf480799d0a115ff1e7da27bd0834ce030914f9c36e9c653845391cc1377a072",
    ".build/project-environments-preview/review-5/provider-claim.json": "c48455b015efbf09b929cc0e014213283451ece22a3b150f16536032c6ac0fc7",
    ".build/project-environments-preview/review-5/cumulative-usage.json": "6863ccb4e79eaeb84131e20322a117265d2d2507660f8609e477e646e6c44500",
    ".build/project-environments-preview/review-5/packaging-review.json": "1fccc16bf5eb8aad77875dc2b8d2661339b6aa25547c5bf208d847d1dc727dfb",
    ".build/project-environments-preview/review-5/provider/attempts.jsonl": "6a5cbd7c4b77b9850f3c10a8879f961d800819ff6fda1011fcea8bd59b4d0ba8",
    ".build/project-environments-preview/review-5/provider/private-output.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/review-5/provider/meter-errors.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/review-5/provider/work/sst/frontend-hosting-artifact.json": "5d9f31e6c3404c3cb956f8b5db2d9c4e9797b3b64dcee8bde9a5f0f6b4a79230",
    ".build/project-environments-preview/review-5/provider/work/sst/pulumi/fe5e7b5733952fb9878de32b/eventlog.json": "31f611ce30be7bc7e6db1a9678223aef20c683d263c7fc2ae73b72d0758332b6",
    ".build/project-environments-preview/review-5/provider/work/sst/log/pulumi.log": "7497186ba072fc862dfcf8995a3e45784f0244a2ba77a12c01934c29a7eb3bca",
    ".build/project-environments-preview/review-5/provider/work/sst/log/pulumi.err.log": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/window-from-168.json": "c1300008f5959c0ec7ee88dce165b749169a7ea4030795819ea97245bec9da2e",
    "docs/development/project-environments-refresh-preview-review-5.json": "3caf9a2ee8ed94a2740919d1cbf51e4bca51f24e62d1b5c1e9ac6d997ef3599b",
    "docs/development/project-environments-refresh-preview-review-5-auth-projection.jsonl": "272d7399d929bb87b80d0c42fedc2822a3ca115cd7216ded1473b7a667a10b03",
    "docs/development/project-environments-refresh-preview-review-5-provider-projection.jsonl": "f88a1b84695ee6d32a890b34df4630b8228ea665c9bbed8ba48f7181f04d0ca7",
}


def baseline_200(root):
    previous = baseline_168(root)
    for name, expected in BASELINE_200_FILES.items():
        if hashlib.sha256((root / name).read_bytes()).hexdigest() != expected:
            raise ValueError("current-accounting-proof-hash")
    run = root / ".build/project-environments-preview/review-5"
    auth = [
        json.loads(x) for x in (run / "authentication.jsonl").read_text().splitlines()
    ]
    provider = [
        json.loads(x)
        for x in (run / "provider/attempts.jsonl").read_text().splitlines()
    ]
    usage = json.loads((run / "cumulative-usage.json").read_text())
    attempts = [x["count"] for x in auth + provider if x["event"] == "attempt"]
    outcomes = [x["attempt"] for x in auth + provider if x["event"] == "outcome"]
    if (
        previous["prior"] != 168
        or attempts != list(range(169, 201))
        or sorted(outcomes) != attempts
        or auth[-1]["event"] != "authenticated"
        or auth[-1]["identityVerified"] is not True
        or provider[-1]["event"] != "stop"
        or provider[-1]["reason"] != "endpoint-held"
        or provider[-1]["count"] != BASELINE_200_PRIOR
        or usage["priorCalls"] != 168
        or usage["newCalls"] != 32
        or usage["cumulativeCalls"] != BASELINE_200_PRIOR
        or usage["ledgerIntegrity"] is not True
    ):
        raise ValueError("stopped-review-5-accounting")
    return {
        **previous,
        "prior": BASELINE_200_PRIOR,
        "review5AuthenticationSha256": BASELINE_200_FILES[
            ".build/project-environments-preview/review-5/authentication.jsonl"
        ],
        "review5ProviderSha256": BASELINE_200_FILES[
            ".build/project-environments-preview/review-5/provider/attempts.jsonl"
        ],
        "review5CampaignClaimSha256": BASELINE_200_FILES[
            ".build/project-environments-preview/window-from-168.json"
        ],
    }


# Review-6 reused cached credentials: one verified STS attempt + thirty reads.
# The private hostname receipt/review sidecar must never be hashed or published.
BASELINE_231_PRIOR = 231
BASELINE_231_FILES = {
    **BASELINE_200_FILES,
    ".build/project-environments-preview/review-6/authentication.jsonl": "0114634ea8e4b0055ccdb6da2427101272ce6ab15e76bb5a6b0f27cfef209407",
    ".build/project-environments-preview/review-6/window.json": "6f768122035d9dcd116e0d8681cfcdf729fa5e830dc41977a578248a0b96f831",
    ".build/project-environments-preview/review-6/handoff.json": "f20980c41e590419fd0f31300013cc87173578c3317690bce63e94c2e0e9a7f1",
    ".build/project-environments-preview/review-6/provider-claim.json": "00002d02bf2f1005cae24f9676d0bd3662731ed2ae200e55549aaddc59985a82",
    ".build/project-environments-preview/review-6/cumulative-usage.json": "6edb2ee9a393bc902551453cf25e44c4b7eb8b8493f0d0331aa5faf89029f27e",
    ".build/project-environments-preview/review-6/packaging-review.json": "1fccc16bf5eb8aad77875dc2b8d2661339b6aa25547c5bf208d847d1dc727dfb",
    ".build/project-environments-preview/review-6/provider/attempts.jsonl": "36944d81c58f361ff0452d907d0f57d40194f4e97b4effbab268ebd55d61d037",
    ".build/project-environments-preview/review-6/provider/config.json": "1b4bd7160afb91fdffbff7f0e30b1864bb5f84d639ce67da6ed2b316cfaaedaf",
    ".build/project-environments-preview/review-6/provider/private-output.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/review-6/provider/meter-errors.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/review-6/provider/work/sst/frontend-hosting-artifact.json": "5d9f31e6c3404c3cb956f8b5db2d9c4e9797b3b64dcee8bde9a5f0f6b4a79230",
    ".build/project-environments-preview/review-6/provider/work/sst/pulumi/fe5e7a5444a57558a228dfee/eventlog.json": "4c6d00c499928e87d10723da591aa52906f3485609bc249421f8d5c3f0dfb888",
    ".build/project-environments-preview/review-6/provider/work/sst/log/pulumi.log": "c750f97f6924523edbdaa67de2a8580687f9dc06a417f13dd060cf6c538632ff",
    ".build/project-environments-preview/review-6/provider/work/sst/log/pulumi.err.log": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/window-from-200.json": "29112244171d5eca5153a52bd8ed607c3e34a43df58b0e00cf95dba43e2e4a10",
    "docs/development/project-environments-refresh-preview-review-6.json": "e192e3b0e2b0c10748e3af9198bb9e5163ccb180250c70169c15b71fb936cd1c",
    "docs/development/project-environments-refresh-preview-review-6-auth-projection.jsonl": "7789344050f7b9c1b0b34dfbd24bcd81921dc04cedc81cf7e8d383312027a9ea",
    "docs/development/project-environments-refresh-preview-review-6-provider-projection.jsonl": "ae57394cd8df9aa34966d9d7e007d3ab1f0b2177b9f54069596a7d3e004b77c4",
}


def baseline_231(root):
    previous = baseline_200(root)
    for name, expected in BASELINE_231_FILES.items():
        if hashlib.sha256((root / name).read_bytes()).hexdigest() != expected:
            raise ValueError("current-accounting-proof-hash")
    run = root / ".build/project-environments-preview/review-6"
    auth = [
        json.loads(x) for x in (run / "authentication.jsonl").read_text().splitlines()
    ]
    provider = [
        json.loads(x)
        for x in (run / "provider/attempts.jsonl").read_text().splitlines()
    ]
    usage = json.loads((run / "cumulative-usage.json").read_text())
    attempts = [x["count"] for x in auth + provider if x["event"] == "attempt"]
    outcomes = [x["attempt"] for x in auth + provider if x["event"] == "outcome"]
    auth_ops = [(x["service"], x["operation"]) for x in auth if x["event"] == "attempt"]
    if (
        previous["prior"] != 200
        or attempts != list(range(201, 232))
        or sorted(outcomes) != attempts
        or auth_ops != [("sts", "GetCallerIdentity")]
        or auth[-1]["event"] != "authenticated"
        or auth[-1]["identityVerified"] is not True
        or provider[-1]["event"] != "stop"
        or provider[-1]["reason"] != "endpoint-held"
        or provider[-1]["count"] != BASELINE_231_PRIOR
        or usage["priorCalls"] != 200
        or usage["newCalls"] != 31
        or usage["cumulativeCalls"] != BASELINE_231_PRIOR
        or usage["ledgerIntegrity"] is not True
    ):
        raise ValueError("stopped-review-6-accounting")
    return {
        **previous,
        "prior": BASELINE_231_PRIOR,
        "review6AuthenticationSha256": BASELINE_231_FILES[
            ".build/project-environments-preview/review-6/authentication.jsonl"
        ],
        "review6ProviderSha256": BASELINE_231_FILES[
            ".build/project-environments-preview/review-6/provider/attempts.jsonl"
        ],
        "review6CampaignClaimSha256": BASELINE_231_FILES[
            ".build/project-environments-preview/window-from-200.json"
        ],
    }


# One existing-session refresh, verified STS, and thirty provider reads in review-7.
# Raw private operation/hostname receipts remain excluded from every digest list.
CURRENT_PRIOR = 263
CURRENT_FROZEN_FILES = {
    **BASELINE_231_FILES,
    ".build/project-environments-preview/review-7/authentication.jsonl": "7d44ce1d4a9b68c53c7d4ab1a5638e919c49ba52871964dd38409eeff309016f",
    ".build/project-environments-preview/review-7/window.json": "970c68deb10d12fcadb0de9ac8b77ef342e2e2181c7fb41eedb129cd858152ed",
    ".build/project-environments-preview/review-7/handoff.json": "d0b328fc205327aee551e7e14d16927e1c742ae1ced3670ed42d5c55b2bf0dab",
    ".build/project-environments-preview/review-7/provider-claim.json": "41ebb96eebfbf225a16f2e8ed03ed15a73a80ce2003376b35c2b1172e0feedbe",
    ".build/project-environments-preview/review-7/cumulative-usage.json": "32657dde4b27d69a76d069e13834910f590fef1fdd2d4606a5bd577e6e41c097",
    ".build/project-environments-preview/review-7/packaging-review.json": "1fccc16bf5eb8aad77875dc2b8d2661339b6aa25547c5bf208d847d1dc727dfb",
    ".build/project-environments-preview/review-7/provider/attempts.jsonl": "9b7c95f02a29286aeb6672fa5fb2d196a9252cebea921f48f81a6e78a1bf9535",
    ".build/project-environments-preview/review-7/provider/config.json": "e164892d65f382b55fa33f2ad8dd9add1950c92f8d8cc7ffdf2f2acefbc90e0e",
    ".build/project-environments-preview/review-7/provider/private-output.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/review-7/provider/meter-errors.txt": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/review-7/provider/work/sst/frontend-hosting-artifact.json": "5d9f31e6c3404c3cb956f8b5db2d9c4e9797b3b64dcee8bde9a5f0f6b4a79230",
    ".build/project-environments-preview/review-7/provider/work/sst/pulumi/fe5e7a267ae65dc9ddca0788/eventlog.json": "aa14d95ce52f50ec60f03f89b60654559b3e445453f709e54ebf0bb886f1f8aa",
    ".build/project-environments-preview/review-7/provider/work/sst/log/pulumi.log": "437375365ff101812efe06af2fee2de63bf35e35d1a4bd4396d51bb52f98df37",
    ".build/project-environments-preview/review-7/provider/work/sst/log/pulumi.err.log": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ".build/project-environments-preview/window-from-231.json": "22a515a776070f0d94fadd55eef3f284e7d9afe344cc8cd1b9960001dbd8268e",
    "docs/development/project-environments-refresh-preview-review-7.json": "22d2a3089288770100901569c0447c4a45db53ee6703fe2e685ef2f33bdc0e95",
    "docs/development/project-environments-refresh-preview-review-7-auth-projection.jsonl": "f64593f835df39316cd6d9371fb5d9d06dcc8f29f85cbbf1ea34a812ff259cb8",
    "docs/development/project-environments-refresh-preview-review-7-provider-projection.jsonl": "072e866035a36a7a93f3e0ddcc31618fb067c51364e97e779a19d0b39fc05e51",
}


def current_baseline(root):
    previous = baseline_231(root)
    for name, expected in CURRENT_FROZEN_FILES.items():
        if hashlib.sha256((root / name).read_bytes()).hexdigest() != expected:
            raise ValueError("current-accounting-proof-hash")
    run = root / ".build/project-environments-preview/review-7"
    auth = [
        json.loads(x) for x in (run / "authentication.jsonl").read_text().splitlines()
    ]
    provider = [
        json.loads(x)
        for x in (run / "provider/attempts.jsonl").read_text().splitlines()
    ]
    usage = json.loads((run / "cumulative-usage.json").read_text())
    attempts = [x["count"] for x in auth + provider if x["event"] == "attempt"]
    outcomes = [x["attempt"] for x in auth + provider if x["event"] == "outcome"]
    auth_ops = [(x["service"], x["operation"]) for x in auth if x["event"] == "attempt"]
    stops = [x for x in provider if x["event"] == "stop"]
    if (
        previous["prior"] != 231
        or attempts != list(range(232, 264))
        or sorted(outcomes) != attempts
        or auth_ops != [("signin", "CreateOAuth2Token"), ("sts", "GetCallerIdentity")]
        or auth[-1]["event"] != "authenticated"
        or auth[-1]["identityVerified"] is not True
        or len(stops) != 1
        or stops[0]["reason"] != "s3control-operation-held"
        or stops[0]["count"] != CURRENT_PRIOR
        or usage["priorCalls"] != 231
        or usage["newCalls"] != 32
        or usage["cumulativeCalls"] != CURRENT_PRIOR
        or usage["ledgerIntegrity"] is not True
    ):
        raise ValueError("stopped-review-7-accounting")
    return {
        **previous,
        "prior": CURRENT_PRIOR,
        "review7AuthenticationSha256": CURRENT_FROZEN_FILES[
            ".build/project-environments-preview/review-7/authentication.jsonl"
        ],
        "review7ProviderSha256": CURRENT_FROZEN_FILES[
            ".build/project-environments-preview/review-7/provider/attempts.jsonl"
        ],
        "review7CampaignClaimSha256": CURRENT_FROZEN_FILES[
            ".build/project-environments-preview/window-from-231.json"
        ],
    }
