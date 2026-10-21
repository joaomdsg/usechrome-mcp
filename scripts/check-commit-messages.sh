#!/usr/bin/env bash
# Fails when a commit message breaks the rules: a single line of at most 72
# characters, capitalised, imperative, no trailing period and no attribution
# trailers. History is the changelog, so it is gated like code. It checks the
# commits in a revision range or, with -m, one message not yet committed.
set -euo pipefail

status=0

check() {
	local label=$1 msg=$2 subject first p
	local problems=()
	subject=$(head -n1 <<<"$msg")
	first=${subject%% *}
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
		echo "::error::$label \"$subject\" $p"
		status=1
	done
}

usage="usage: check-commit-messages.sh <revision-range> | -m <message>"
if [[ ${1:?$usage} == -m ]]; then
	check message "${2:?$usage}"
	exit $status
fi

# Assigned first so a bad range fails the script; set -e skips the list of a for.
shas=$(git rev-list "$1")
for sha in $shas; do
	check "${sha:0:12}" "$(git log -1 --format=%B "$sha")"
done

exit $status
