#!/usr/bin/env bash
# GoReleaser's post-build hook. The release rebuilds every platform, so it
# fails when its linux/amd64 build differs by a byte from the binary that the
# acceptance stage tested (TESTED_BIN): a release ships only what passed.
# Outside the release job TESTED_BIN is unset and the hook does nothing.
set -euo pipefail

built=${1:?usage: check-release-binary.sh <built-binary> <target>}
target=${2:?usage: check-release-binary.sh <built-binary> <target>}

[[ -n ${TESTED_BIN:-} && $target == linux_amd64_v1 ]] || exit 0

if ! cmp -s "$built" "$TESTED_BIN"; then
	echo "::error::the release build $built differs from the tested binary $TESTED_BIN; check that make build and .goreleaser.yaml use the same Go and flags" >&2
	exit 1
fi
echo "release build $built is the tested binary"
