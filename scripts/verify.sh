#!/bin/sh
# Taskfile and CI call this same product verification contract.
set -eu

packages() {
    module=$(go list -m)
    listed=$(go list -e ./...)
    printf '%s\n' "$listed" | awk -v excluded="$module/context" '$0 != excluded && index($0, excluded "/") != 1'
}

module=$(go list -m)
scope=$(packages)
[ -n "$scope" ] || { echo "FAIL: empty product scope" >&2; exit 1; }

lint_packages() {
    printf '%s\n' "$scope" | awk -v module="$module" '{
        if ($0 == module) print "."
        else print "./" substr($0, length(module) + 2)
    }'
}

case "${1:-}" in
    packages) printf '%s\n' "$scope" ;;
    lint-packages) lint_packages ;;
    focused)
        shift
        selected_packages=
        run_pattern=
        run_pattern_set=0
        while [ "$#" -gt 0 ]; do
            case "$1" in
                --run)
                    shift
                    [ "$#" -gt 0 ] || { echo "FAIL: --run requires a Go test pattern" >&2; exit 2; }
                    run_pattern=$1
                    run_pattern_set=1
                    shift
                    [ "$#" -eq 0 ] || { echo "FAIL: --run must follow all package paths" >&2; exit 2; }
                    ;;
                -* ) echo "FAIL: unsupported focused-test argument: $1" >&2; exit 2 ;;
                *)
                    case "$1" in
                        .|./) package_path=$module ;;
                        ./*) package_path="$module/${1#./}" ;;
                        *) package_path=$1 ;;
                    esac
                    if ! printf '%s\n' "$scope" | awk -v selected="$package_path" '$0 == selected { found = 1 } END { exit !found }'; then
                        echo "FAIL: package is not in the product scope: $1" >&2
                        exit 2
                    fi
                    selected_packages="${selected_packages}${selected_packages:+ }$package_path"
                    shift
                    ;;
            esac
        done
        [ -n "$selected_packages" ] || { echo "FAIL: specify at least one product package" >&2; exit 2; }

        # Package tokens were matched exactly against `go list` output; disable
        # globbing before rebuilding the positional arguments for go test.
        set -f
        set -- $selected_packages
        printf 'Focused Go tests; packages: %s\n' "$selected_packages"
        printf '%s\n' 'Not run: repository-wide coverage, unselected packages, full delivery gate.'
        if [ "$run_pattern_set" -eq 1 ]; then
            printf 'Go test pattern: %s\n' "$run_pattern"
            go test -run "$run_pattern" "$@"
        else
            go test "$@"
        fi
        ;;
    test) go test -covermode=atomic -coverprofile=coverage.out $scope ;;
    build) go build $scope ;;
    vet) go vet $scope ;;
    lint) golangci-lint run $(lint_packages) ;;
    vuln) govulncheck $scope ;;
    fmt)
        directories=$(go list -f '{{.Dir}}' $scope)
        files=$(printf '%s\n' "$directories" | while IFS= read -r dir; do
            gofmt -l "$dir"/*.go
        done)
        if [ -n "$files" ]; then printf '%s\n' "$files"; exit 1; fi
        ;;
    modules)
        go mod tidy -diff
        go mod verify
        ;;
    coverage)
        expected=$(printf '%s\n' "$scope" | paste -sd ' ' -)
        awk -v expected="$expected" -v module="$(go list -m)" '
        BEGIN {
            split(expected, list, " ")
            for (i in list) wanted[list[i]] = 1
            floor[module "/cli/utils"] = 18.1
            floor[module "/internal/acpfixture"] = 66.7
            floor[module "/internal/feedback"] = 77.7
            floor[module "/internal/figure"] = 77.0
        }
        NR == 1 { if ($0 !~ /^mode: (set|count|atomic)$/) bad = 1; next }
        {
            if (NF != 3 || $2 !~ /^[0-9]+$/ || $3 !~ /^[0-9]+$/) { bad = 1; next }
            pkg = $1
            sub(/\/[^\/]+$/, "", pkg)
            if (!(pkg in wanted)) { print "FAIL: unexpected package " pkg; bad = 1 }
            total[pkg] += $2
            if ($3 > 0) covered[pkg] += $2
        }
        END {
            for (pkg in wanted) {
                # The two main packages are verified by build/install/metadata checks.
                if (pkg == module "/internal/acpbridge/acptest") {
                    print "EXCLUDED: " pkg " (test peer; acpbridge consumer tests required)"
                    continue
                }
                if (pkg == module || pkg == module "/cli") {
                    print "EXCLUDED: " pkg " (entry point; build/install checks)"
                    continue
                }
                if (!total[pkg]) { print "FAIL: missing coverage for " pkg; bad = 1; continue }
                value = sprintf("%.1f", 100 * covered[pkg] / total[pkg])
                minimum = (pkg in floor) ? floor[pkg] : 70
                if (value + 0 < minimum) {
                    printf "FAIL: %s %s%% < %.1f%%\n", pkg, value, minimum
                    bad = 1
                } else printf "PASS: %s %s%% (floor %.1f%%)\n", pkg, value, minimum
            }
            exit bad
        }' "${2:-coverage.out}"
        ;;
    *) echo 'usage: sh scripts/verify.sh packages|focused <package>... [--run pattern]|test|coverage [profile]|build|vet|lint|vuln|fmt|modules' >&2; exit 2 ;;
esac
