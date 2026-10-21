#!/usr/bin/env bash
# Runs the release image as if it were the binary, so `make acceptance-image`
# can name this script as USECHROME_MCP_BIN. Host networking lets Chromium in
# the container reach the fixture servers the tests start on loopback.
set -euo pipefail

exec docker run --rm -i --network host "${USECHROME_MCP_IMAGE:?set USECHROME_MCP_IMAGE to the image to run}" "$@"
