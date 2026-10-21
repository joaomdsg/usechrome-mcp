#!/usr/bin/env bash
# Prints the one-line message that squash-merging a Dependabot pull request
# titled $1 puts on main. Its titles end in " in /<directory>", which pushes
# many past the 72-character limit; the diff names the directory, so a title
# that is too long drops it.
set -euo pipefail

title=${1:?usage: squash-subject.sh <pull request title>}
if ((${#title} > 72)); then
	title=${title% in /*}
fi
printf '%s\n' "$title"
