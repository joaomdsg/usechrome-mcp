#!/usr/bin/env bash
# Prints each mutant in the gomutants JSON report $1 whose status is one of the
# rest of the arguments. Efficacy, KILLED/(KILLED+LIVED), leaves TIMED OUT
# mutants out, and gomutants' memory cap kills some that way while logging only
# their package: it over-counts Chrome's shared memory
# (github.com/gomutants/gomutants/issues/127). This is where a reader sees
# which ones dropped out of the score, and which ones survived it.
set -euo pipefail

report=${1:?usage: list-mutants.sh <gomutants JSON report> <status>...}
shift
statuses=$(jq -cn '$ARGS.positional' --args "$@")
jq -r --argjson s "$statuses" '.files[]?.mutations[]? | select(.status as $x | $s | index($x)) | "\(.status) \(.id) line \(.line)"' "$report"
