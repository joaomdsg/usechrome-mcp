#!/usr/bin/env bash
# Prints each TIMED OUT mutant in the gomutants JSON report $1. Efficacy,
# KILLED/(KILLED+LIVED), leaves them out, and gomutants logs a memory-cap
# kill by package only, so this is where a reader sees which ones dropped out
# of the score.
set -euo pipefail

report=${1:?usage: list-timed-out.sh <gomutants JSON report>}
jq -r '.files[]?.mutations[]? | select(.status == "TIMED OUT") | "TIMED OUT \(.id) line \(.line)"' "$report"
