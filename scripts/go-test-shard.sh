#!/bin/sh
# Splits the Go tests into the CI's numbered shards.
#
#   go-test-shard.sh <shard> <shards> list           prints the shard's packages, one a line
#   go-test-shard.sh <shard> <shards> test [flag...]  runs go test with the flags on the shard's tests
#
# Each line of the table is a shard, by measured time. The last shard is the one after them: it runs every package
# that no line names, so a new package runs there. A package that alone takes longer than a shard is split by its tests:
# a line of the package and a part i/k, such as "./pkg 1/2", runs the i-th of every k tests in the order go lists them.
# Every run checks the table against the packages there are and the shard count the workflow gives.
set -eu

table='
./desktop/threads 1/2
./desktop/threads 2/2
./desktop/browser ./desktop/terminal ./internal/host ./desktop/update
./desktop/orchestration ./desktop/issues ./desktop/chat ./desktop/internal/subcommand
'

fail() {
	echo "go-test-shard: $*" >&2
	exit 1
}

[ $# -ge 3 ] || fail "usage: go-test-shard.sh <shard> <shards> list|test [go test flag...]"
shard=$1 shards=$2 mode=$3
shift 3
case $mode in list | test) ;; *) fail "no mode $mode" ;; esac

lines=$(printf '%s\n' "$table" | sed '/^$/d')
count=$(($(printf '%s\n' "$lines" | wc -l) + 1))
[ "$shards" -eq "$count" ] || fail "the workflow runs $shards shards, the table makes $count"
[ "$shard" -ge 1 ] && [ "$shard" -le "$count" ] || fail "no shard $shard of $count"

printf '%s\n' "$lines" | awk '
	$NF ~ /^[0-9]+\/[0-9]+$/ {
		split($NF, p, "/")
		if (NF != 2 || ($1 in parts && parts[$1] != p[2])) bad = 1
		parts[$1] = p[2]
		seen[$1, p[1]]++
		next
	}
	{ for (i = 1; i <= NF; i++) whole[$i]++ }
	END {
		for (pkg in whole) if (whole[pkg] > 1 || pkg in parts) bad = 1
		for (pkg in parts) for (i = 1; i <= parts[pkg]; i++) if (seen[pkg, i] != 1) bad = 1
		exit bad
	}' || fail "the table runs a package twice, or a split package misses a part or shares its line"

all=$(go list -tags stub ./... | sed 's|^github.com/nednella/orchestrator|.|')
named=$(printf '%s\n' "$lines" | tr ' ' '\n' | grep -Ev '^[0-9]+/[0-9]+$' | sort -u)
for pkg in $named; do
	printf '%s\n' "$all" | grep -qxF "$pkg" || fail "the table names $pkg, which is no package"
done

if [ "$shard" -lt "$count" ]; then
	line=$(printf '%s\n' "$lines" | sed -n "${shard}p")
else
	line=$(printf '%s\n' "$all" | grep -vxF "$named" | tr '\n' ' ')
fi

part=${line##* }
case $part in
[0-9]*/[0-9]*)
	pkg=${line% *}
	if [ "$mode" = list ]; then
		echo "$pkg"
		exit 0
	fi
	# go test -list names the top-level tests, examples and fuzz tests, and then the package.
	tests=$(go test "$@" -list . "$pkg" | grep -E '^(Test|Example|Fuzz)' |
		awk -v i="${part%/*}" -v k="${part#*/}" '(NR - 1) % k == i - 1' | paste -sd '|' -)
	exec go test "$@" -run "^($tests)\$" "$pkg"
	;;
esac

if [ "$mode" = list ]; then
	printf '%s\n' $line
else
	exec go test "$@" $line
fi
