#!/usr/bin/env bash
# Fails when KILLED/(KILLED+LIVED+TIMED OUT) in the gomutants JSON report $1
# is below $2 percent. gomutants' own efficacy leaves TIMED OUT out, and its
# memory cap turns hundreds of internal/chrome mutants into TIMED OUT on a
# GitHub runner (github.com/gomutants/gomutants/issues/127): the first weekly
# run scored 93.8% that way against 81.5% locally. Counting them as survivors
# keeps cap kills from raising the score.
set -euo pipefail

report=${1:?usage: mutation-score.sh <gomutants JSON report> <floor percent>}
floor=${2:?usage: mutation-score.sh <gomutants JSON report> <floor percent>}
jq -r --argjson floor "$floor" '
  [.files[]?.mutations[]?.status] as $s
  | ($s | map(select(. == "KILLED")) | length) as $k
  | ($s | map(select(. == "LIVED")) | length) as $l
  | ($s | map(select(. == "TIMED OUT")) | length) as $t
  | ($k * 10000 / ($k + $l + $t) | floor / 100) as $score
  | "killed \($k), lived \($l), timed out \($t): \($score)% against a floor of \($floor)%",
    if $score < $floor then error("below the floor") else empty end
' "$report"
