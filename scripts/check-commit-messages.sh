#!/usr/bin/env bash
# Fails when a commit in the range breaks the message rules: a single line
# of at most 72 characters, capitalised, imperative, no trailing period and
# no attribution trailers. History is the changelog, so it is gated like code.
set -euo pipefail

range=${1:?usage: check-commit-messages.sh <revision-range>}
status=0
# Assigned first so a bad range fails the script; set -e skips the list of a for.
shas=$(git rev-list "$range")

for sha in $shas; do
	msg=$(git log -1 --format=%B "$sha")
	subject=$(head -n1 <<<"$msg")
	first=${subject%% *}
	problems=()
	(($(grep -c . <<<"$msg") == 1)) || problems+=("has more than one line")
	((${#subject} <= 72)) || problems+=("is longer than 72 characters")
	[[ $subject =~ ^[A-Z] ]] || problems+=("does not start with a capital letter")
	[[ $subject != *. ]] || problems+=("ends with a period")
	# Past tense, gerunds and third person ("Added", "Adding", "Adds") are
	# not imperative; the exceptions are verbs that merely end that way.
	if [[ $first =~ (ed|ing|[^su]s)$ && ! $first =~ ^(Embed|Feed|Seed|Speed|Shed|Need|Proceed|Exceed|Bring|Ping|String|Bias)$ ]]; then
		problems+=("is not imperative (\"$first\")")
	fi
	for p in "${problems[@]}"; do
		echo "::error::${sha:0:12} \"$subject\" $p"
		status=1
	done
done

exit $status
