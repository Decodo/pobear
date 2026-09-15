#!/bin/sh

set -eu

SCRIPT="${1:-install.sh}"
[ -f "$SCRIPT" ] || {
	echo "usage: sh test/install_test.sh [path/to/install.sh]" >&2
	exit 2
}
SCRIPT=$(cd "$(dirname "$SCRIPT")" && pwd)/$(basename "$SCRIPT")

command -v python3 >/dev/null 2>&1 || {
	echo "install_test: python3 is required to serve fixtures" >&2
	exit 2
}

PORT="${INSTALL_TEST_PORT:-8973}"
VERSION="9.9.9"
ROOT=$(mktemp -d)
SRV=""
trap 'rm -rf "$ROOT"; [ -n "$SRV" ] && kill "$SRV" 2>/dev/null; exit' EXIT INT TERM

PASS=0
FAIL=0

ok() {
	PASS=$((PASS + 1))
	printf '  ok    %s\n' "$1"
}

bad() {
	FAIL=$((FAIL + 1))
	printf '  FAIL  %s\n        %s\n' "$1" "$2"
}

check() {
	if [ "$2" = "$3" ]; then
		ok "$1"
	else
		bad "$1" "expected '$3', got '$2'"
	fi
}

contains() {
	case "$2" in
	*"$3"*) ok "$1" ;;
	*) bad "$1" "expected output to contain '$3', got: $2" ;;
	esac
}

assert() {
	if [ "$2" = "0" ]; then
		ok "$1"
	else
		bad "$1" "$3"
	fi
}

lacks() {
	case "$2" in
	*"$3"*) bad "$1" "expected output NOT to contain '$3', got: $2" ;;
	*) ok "$1" ;;
	esac
}

build_fixtures() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	case "$(uname -m)" in
	x86_64 | amd64) arch="amd64" ;;
	*) arch="arm64" ;;
	esac
	ASSET="pobear_${VERSION}_${os}_${arch}.tar.gz"

	mkdir -p "$ROOT/mirror" "$ROOT/stage"
	printf '#!/bin/sh\necho "pobear %s"\n' "$VERSION" >"$ROOT/stage/pobear"
	chmod +x "$ROOT/stage/pobear"
	tar -czf "$ROOT/mirror/$ASSET" -C "$ROOT/stage" pobear

	mkdir -p "$ROOT/stage-broken"
	printf 'not a binary' >"$ROOT/stage-broken/pobear"
	chmod +x "$ROOT/stage-broken/pobear"
	BROKEN_ASSET="pobear_8.8.8_${os}_${arch}.tar.gz"
	tar -czf "$ROOT/mirror/$BROKEN_ASSET" -C "$ROOT/stage-broken" pobear

	# goreleaser writes bare filenames; strip the ./ the glob introduces.
	(cd "$ROOT/mirror" && sums ./*.tar.gz | sed 's|\./||' >checksums.txt)
	printf '{"tag_name":"v%s"}' "$VERSION" >"$ROOT/mirror/latest.json"
}

sums() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$@"
	else
		shasum -a 256 "$@"
	fi
}

start_server() {
	(cd "$ROOT/mirror" && exec python3 -m http.server "$PORT" >/dev/null 2>&1) &
	SRV=$!
	i=0
	while [ "$i" -lt 50 ]; do
		if curl -fsS -o /dev/null "http://127.0.0.1:$PORT/checksums.txt" 2>/dev/null; then
			return 0
		fi
		i=$((i + 1))
		sleep 0.1
	done
	echo "install_test: fixture server did not start on port $PORT" >&2
	exit 2
}

patch_script() {
	sed -e "s|https://api.github.com/repos/\$REPO/releases/latest|http://127.0.0.1:$PORT/latest.json|" \
		-e "s|https://github.com/\$REPO/releases/download/\$TAG|http://127.0.0.1:$PORT|" \
		"$SCRIPT" >"$ROOT/install.sh"
	n=$(grep -c "127.0.0.1:$PORT" "$ROOT/install.sh" || true)
	[ "$n" = "2" ] || {
		echo "install_test: expected to rewrite 2 URLs in install.sh, rewrote $n" >&2
		echo "install_test: the script's download endpoints changed shape; update patch_script" >&2
		exit 2
	}
}

run_install() {
	case_dir="$ROOT/cases/$1"
	shift
	rm -rf "$case_dir"
	mkdir -p "$case_dir/home" "$case_dir/pathdir" "$case_dir/target"
	env -i \
		HOME="$case_dir/home" \
		PATH="$case_dir/pathdir:/usr/bin:/bin:/usr/sbin:/sbin" \
		SHELL=/bin/zsh \
		POBEAR_INSTALL_DIR="$case_dir/target" \
		POBEAR_VERSION="$VERSION" \
		"$@" \
		sh "$ROOT/install.sh" 2>&1
}

echo "install.sh test suite"
build_fixtures
start_server
patch_script

CASE="$ROOT/cases"

out=$(run_install happy || true)
contains "installs and reports the version" "$out" "pobear $VERSION"
check "binary is executable" "$("$CASE/happy/target/pobear" version)" "pobear $VERSION"
lacks "no leftover .tmp file" "$(find "$CASE/happy/target" -name "*.tmp" -print)" ".tmp"

out=$(run_install happy2 || true)
out=$(cd "$CASE/happy2" && env -i HOME="$CASE/happy2/home" PATH="$CASE/happy2/pathdir:/usr/bin:/bin" \
	SHELL=/bin/zsh POBEAR_INSTALL_DIR="$CASE/happy2/target" POBEAR_VERSION="$VERSION" \
	sh "$ROOT/install.sh" 2>&1 || true)
contains "reinstall succeeds" "$out" "pobear $VERSION"
check "reinstall leaves one file" "$(find "$CASE/happy2/target" -type f | wc -l | tr -d ' ')" "1"

out=$(rm -rf "$CASE/tag" && mkdir -p "$CASE/tag/home" "$CASE/tag/pathdir" "$CASE/tag/target" &&
	env -i HOME="$CASE/tag/home" PATH="$CASE/tag/pathdir:/usr/bin:/bin" SHELL=/bin/zsh \
		POBEAR_INSTALL_DIR="$CASE/tag/target" sh "$ROOT/install.sh" 2>&1 || true)
contains "resolves the latest tag when POBEAR_VERSION is unset" "$out" "v$VERSION"

cp "$ROOT/mirror/checksums.txt" "$ROOT/checksums.bak"
sed 's/^[0-9a-f]\{64\}/0000000000000000000000000000000000000000000000000000000000000000/' \
	"$ROOT/checksums.bak" >"$ROOT/mirror/checksums.txt"
out=$(run_install badsum || true)
contains "checksum mismatch is rejected" "$out" "checksum mismatch"
check "nothing installed on mismatch" "$(find "$CASE/badsum/target" -type f | wc -l | tr -d ' ')" "0"
cp "$ROOT/checksums.bak" "$ROOT/mirror/checksums.txt"

grep -v "$ASSET" "$ROOT/checksums.bak" >"$ROOT/mirror/checksums.txt"
out=$(run_install nosum || true)
contains "asset missing from checksums.txt is rejected" "$out" "not listed in checksums.txt"
cp "$ROOT/checksums.bak" "$ROOT/mirror/checksums.txt"

out=$(rm -rf "$CASE/missing" && mkdir -p "$CASE/missing/home" "$CASE/missing/pathdir" "$CASE/missing/target" &&
	env -i HOME="$CASE/missing/home" PATH="$CASE/missing/pathdir:/usr/bin:/bin" SHELL=/bin/zsh \
		POBEAR_INSTALL_DIR="$CASE/missing/target" POBEAR_VERSION="1.2.3" \
		sh "$ROOT/install.sh" 2>&1 || true)
contains "a 404 does not claim the asset is absent" "$out" "may not exist or the repository may be private"

rm -rf "$CASE/ro"
mkdir -p "$CASE/ro/home" "$CASE/ro/pathdir" "$CASE/ro/target"
chmod 555 "$CASE/ro/target"
out=$(env -i HOME="$CASE/ro/home" PATH="$CASE/ro/pathdir:/usr/bin:/bin" SHELL=/bin/zsh \
	POBEAR_INSTALL_DIR="$CASE/ro/target" POBEAR_VERSION="$VERSION" \
	sh "$ROOT/install.sh" 2>&1 || true)
contains "unwritable directory fails before downloading" "$out" "not writable"
lacks "unwritable directory never downloads" "$out" "Downloading"
chmod 755 "$CASE/ro/target"

rm -rf "$CASE/clash"
mkdir -p "$CASE/clash/home" "$CASE/clash/pathdir" "$CASE/clash/target"
printf '#!/bin/sh\necho "other-tool 1.0"\n' >"$CASE/clash/target/pobear"
chmod +x "$CASE/clash/target/pobear"
out=$(env -i HOME="$CASE/clash/home" PATH="$CASE/clash/pathdir:/usr/bin:/bin" SHELL=/bin/zsh \
	POBEAR_INSTALL_DIR="$CASE/clash/target" POBEAR_VERSION="$VERSION" \
	sh "$ROOT/install.sh" 2>&1 || true)
contains "refuses to overwrite a foreign binary" "$out" "is not pobear"
check "foreign binary survives" "$("$CASE/clash/target/pobear")" "other-tool 1.0"

out=$(env -i HOME="$CASE/clash/home" PATH="$CASE/clash/pathdir:/usr/bin:/bin" SHELL=/bin/zsh \
	POBEAR_INSTALL_DIR="$CASE/clash/target" POBEAR_VERSION="$VERSION" POBEAR_FORCE=1 \
	sh "$ROOT/install.sh" 2>&1 || true)
check "POBEAR_FORCE overwrites it" "$("$CASE/clash/target/pobear" version)" "pobear $VERSION"

rm -rf "$CASE/broken"
mkdir -p "$CASE/broken/home" "$CASE/broken/pathdir" "$CASE/broken/target"
printf 'garbage' >"$CASE/broken/target/pobear"
chmod +x "$CASE/broken/target/pobear"
out=$(env -i HOME="$CASE/broken/home" PATH="$CASE/broken/pathdir:/usr/bin:/bin" SHELL=/bin/zsh \
	POBEAR_INSTALL_DIR="$CASE/broken/target" POBEAR_VERSION="$VERSION" \
	sh "$ROOT/install.sh" 2>&1 || true)
contains "a broken existing install is repaired" "$out" "pobear $VERSION"

out=$(rm -rf "$CASE/norun" && mkdir -p "$CASE/norun/home" "$CASE/norun/pathdir" "$CASE/norun/target" &&
	env -i HOME="$CASE/norun/home" PATH="$CASE/norun/pathdir:/usr/bin:/bin" SHELL=/bin/zsh \
		POBEAR_INSTALL_DIR="$CASE/norun/target" POBEAR_VERSION="8.8.8" \
		sh "$ROOT/install.sh" 2>&1 || true)
contains "a binary that will not run is caught after install" "$out" "will not run"

rm -rf "$CASE/slash"
mkdir -p "$CASE/slash/home" "$CASE/slash/pathdir" "$CASE/slash/target"
out=$(env -i HOME="$CASE/slash/home" PATH="$CASE/slash/target:/usr/bin:/bin" SHELL=/bin/zsh \
	POBEAR_INSTALL_DIR="$CASE/slash/target/" POBEAR_VERSION="$VERSION" \
	sh "$ROOT/install.sh" 2>&1 || true)
lacks "a trailing slash does not cause a false PATH warning" "$out" "not on your PATH"
contains "a trailing slash is normalised in the reported path" "$out" "$CASE/slash/target/pobear"
if [ -L "$CASE/slash/target/pobear" ]; then
	bad "an unnormalised dir does not self-symlink" "pobear in the install dir is a symlink"
else
	ok "an unnormalised dir does not self-symlink"
fi

rm -rf "$CASE/rel"
mkdir -p "$CASE/rel/home" "$CASE/rel/target"
out=$(cd "$CASE/rel" && env -i HOME="$CASE/rel/home" PATH="$CASE/rel/target:/usr/bin:/bin" SHELL=/bin/zsh \
	POBEAR_INSTALL_DIR="./target" POBEAR_VERSION="$VERSION" \
	sh "$ROOT/install.sh" 2>&1 || true)
contains "a relative dir is reported as an absolute path" "$out" "$CASE/rel/target/pobear"

out=$(run_install link || true)
contains "links into a writable directory on PATH" "$out" "Linked into"
if [ -L "$CASE/link/pathdir/pobear" ]; then
	ok "the link is a symlink"
else
	bad "the link is a symlink" "no symlink at $CASE/link/pathdir/pobear"
fi

rm -rf "$CASE/rc"
mkdir -p "$CASE/rc/home" "$CASE/rc/ro" "$CASE/rc/target"
chmod 555 "$CASE/rc/ro"
out=$(env -i HOME="$CASE/rc/home" PATH="$CASE/rc/ro:/usr/bin:/bin" SHELL=/bin/zsh \
	POBEAR_INSTALL_DIR="$CASE/rc/target" POBEAR_VERSION="$VERSION" \
	sh "$ROOT/install.sh" 2>&1 || true)
contains "falls back to editing the shell rc" "$out" "Added"
if grep -Fq "$CASE/rc/target" "$CASE/rc/home/.zshrc" 2>/dev/null; then
	ok "the rc file gained the export line"
else
	bad "the rc file gained the export line" "no PATH line in .zshrc"
fi
chmod 755 "$CASE/rc/ro"

rm -rf "$CASE/nomod"
mkdir -p "$CASE/nomod/home" "$CASE/nomod/ro" "$CASE/nomod/target"
chmod 555 "$CASE/nomod/ro"
out=$(env -i HOME="$CASE/nomod/home" PATH="$CASE/nomod/ro:/usr/bin:/bin" SHELL=/bin/zsh \
	POBEAR_INSTALL_DIR="$CASE/nomod/target" POBEAR_VERSION="$VERSION" POBEAR_NO_MODIFY_PATH=1 \
	sh "$ROOT/install.sh" 2>&1 || true)
contains "POBEAR_NO_MODIFY_PATH prints the hint instead" "$out" "is not on your PATH"
if [ -f "$CASE/nomod/home/.zshrc" ]; then
	bad "POBEAR_NO_MODIFY_PATH leaves the rc alone" "a .zshrc was created"
else
	ok "POBEAR_NO_MODIFY_PATH leaves the rc alone"
fi
chmod 755 "$CASE/nomod/ro"

echo
echo "$PASS passed, $FAIL failed"
[ "$FAIL" = "0" ] || exit 1
