#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmpdir=$(mktemp -d "$root/context/tmp/verify-focused-test.XXXXXX")
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM
mkdir -p "$tmpdir/bin"

cat > "$tmpdir/bin/go" <<'GO'
#!/bin/sh
if [ "$1" = list ] && [ "$2" = -m ]; then
    echo github.com/sandrolain/sdt
elif [ "$1" = list ] && [ "$2" = -e ]; then
    printf '%s\n' \
        github.com/sandrolain/sdt \
        github.com/sandrolain/sdt/internal/mdstruct \
        github.com/sandrolain/sdt/context/fixture
elif [ "$1" = test ]; then
    shift
    printf '%s\n' "$@" > "$GO_ARGS_FILE"
else
    echo "unexpected go invocation: $*" >&2
    exit 90
fi
GO
chmod +x "$tmpdir/bin/go"

PATH="$tmpdir/bin:$PATH" GO_ARGS_FILE="$tmpdir/go-args" \
    task test-focused -- ./internal/mdstruct --run 'Test.*' > "$tmpdir/focused.out"
grep -F 'packages: github.com/sandrolain/sdt/internal/mdstruct' "$tmpdir/focused.out" >/dev/null
grep -F 'Not run: repository-wide coverage, unselected packages, full delivery gate.' "$tmpdir/focused.out" >/dev/null
printf '%s\n' '-run' 'Test.*' 'github.com/sandrolain/sdt/internal/mdstruct' > "$tmpdir/expected-args"
cmp -s "$tmpdir/expected-args" "$tmpdir/go-args" || {
    echo 'FAIL: focused task did not preserve the selected package and test pattern' >&2
    exit 1
}

if PATH="$tmpdir/bin:$PATH" GO_ARGS_FILE="$tmpdir/go-args" \
    task test-focused -- > "$tmpdir/empty.out" 2>&1; then
    echo 'FAIL: focused task accepted an empty package selection' >&2
    exit 1
fi
grep -F 'specify at least one product package' "$tmpdir/empty.out" >/dev/null

if PATH="$tmpdir/bin:$PATH" GO_ARGS_FILE="$tmpdir/go-args" \
    task test-focused -- ./context/fixture > "$tmpdir/context.out" 2>&1; then
    echo 'FAIL: focused task accepted a context fixture package' >&2
    exit 1
fi
grep -F 'package is not in the product scope: ./context/fixture' "$tmpdir/context.out" >/dev/null

echo 'PASS: focused verification scope, pattern forwarding, and rejection cases'
