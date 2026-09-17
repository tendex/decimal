#!/bin/sh
# run.sh runs the benchmarks behind the tables in the README: those of the
# decimal package itself and the comparison with other libraries. It runs
# them in interleaved rounds, so that any drift in the speed of the machine
# affects all benchmarks alike, then records the results in results/ and
# updates the README.
#
#	./run.sh                           # 6 rounds of 250ms per benchmark
#	ROUNDS=10 BENCHTIME=1s ./run.sh
set -eu
cd "$(dirname "$0")"
rounds=${ROUNDS:-6}
benchtime=${BENCHTIME:-250ms}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
(cd .. && go test -c -o "$tmp/decimal.test" .)
go test -c -o "$tmp/compare.test" .

mkdir -p results
for name in decimal compare; do
	printf 'go: %s\nbenchtime: %s\n' "$(go env GOVERSION)" "$benchtime" >"results/$name.txt"
done
i=1
while [ "$i" -le "$rounds" ]; do
	echo "round $i of $rounds" >&2
	for name in decimal compare; do
		"$tmp/$name.test" -test.run '^$' -test.bench . -test.benchmem \
			-test.benchtime "$benchtime" | grep -v '^PASS$' >>"results/$name.txt"
	done
	i=$((i + 1))
done

go run ./cmd/benchtable -readme ../README.md results/decimal.txt results/compare.txt
