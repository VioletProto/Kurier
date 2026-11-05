"""Immutable host payloads for the adopted artifacts; no AWS work."""

import json

from common import ROOT, Held, digest


def load_objects(scope, release):
    objects = {}
    for p in release["packages"]:
        b = (ROOT / release["artifactDirectory"] / p["name"] / "code.zip").read_bytes()
        if len(b) != p["zipBytes"] or digest(b) != p["zipSha256"]:
            raise Held("adopted-archive-held")
        objects[
            (scope["assetBucket"], f"assets/{p['name']}-code-{p['zipSha256']}.zip")
        ] = b
    original = json.loads(
        (ROOT / "docs/development/project-environments-release.json").read_text()
    )
    folder = ROOT / original["artifactDirectory"] / "frontend"
    for entry in scope["frontendObjects"]:
        k = entry["key"]
        source = k
        if k.startswith("_releases/" + release["frontend"]["releaseId"] + "/"):
            source = k.split("/", 2)[2]
        file = folder / "site" / source
        b = file.read_bytes()
        if source != "artifact.json":
            expected = next(
                v for v in release["frontend"]["files"] if v["path"] == source
            )
            if len(b) != expected["bytes"] or digest(b) != expected["sha256"]:
                raise Held("frontend-source-held")
        else:
            if json.loads(b) != release["frontend"]:
                raise Held("frontend-marker-held")
        objects[(scope["webBucket"], k)] = b
    return objects
