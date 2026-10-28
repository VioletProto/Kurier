#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
PYTHONDONTWRITEBYTECODE=1 python3 scripts/preview-meter/test_unit.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/preview-meter/test_diagnostics.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/preview-meter/test_handoff.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/preview-meter/test_auth_transport_diagnostics.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/preview-meter/test_compare_auth_paths.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/preview-meter/test_network_probe.py
# The entire fixture supervisor/proxy/origin has no external network route.
# The production child sandbox is nested inside it and more restrictive.
bwrap --unshare-net --ro-bind / / --bind "$PWD" "$PWD" --bind /tmp /tmp --proc /proc --dev /dev \
  env PYTHONDONTWRITEBYTECODE=1 python3 scripts/preview-meter/test_transport.py
