#!/bin/sh
set -eu

REPO="Decodo/pobear"
BINARY="pobear"
INSTALL_DIR="${POBEAR_INSTALL_DIR:-$HOME/.local/bin}"

LINKED_DIR=""
RC_FILE=""
REPLACED_EXISTING=0

if [ -t 1 ]; then
	BOLD=$(printf '\033[1m')
	DIM=$(printf '\033[2m')
	YELLOW=$(printf '\033[0;33m')
	RESET=$(printf '\033[0m')
else
	BOLD='' DIM='' YELLOW='' RESET=''
fi

die() {
	echo "install: $*" >&2
	exit 1
}

warn() {
	echo "${YELLOW}warning:${RESET} $*" >&2
}

has() {
	command -v "$1" >/dev/null 2>&1
}

fetch() {
	if has curl; then
		curl -fsSL "$1"
	else
		wget -qO- "$1"
	fi
}

download() {
	if has curl; then
		curl -fsSL -o "$2" "$1"
	else
		wget -qO "$2" "$1"
	fi
}

check_prerequisites() {
	has curl || has wget || die "curl or wget is required"
	has tar || die "tar is required"
	has sha256sum || has shasum || die "sha256sum or shasum is required"
}

detect_platform() {
	uname_s=$(uname -s)
	uname_m=$(uname -m)

	case "$uname_s" in
	Linux) OS="linux" ;;
	Darwin) OS="darwin" ;;
	MINGW* | MSYS* | CYGWIN*)
		die "Windows is not supported by this script, download a .zip release from https://github.com/$REPO/releases"
		;;
	*) die "unsupported operating system: $uname_s" ;;
	esac

	case "$uname_m" in
	x86_64 | amd64) ARCH="amd64" ;;
	aarch64 | arm64) ARCH="arm64" ;;
	*) die "unsupported architecture: $uname_m" ;;
	esac
}

resolve_tag() {
	if [ -n "${POBEAR_VERSION:-}" ]; then
		TAG="v${POBEAR_VERSION#v}"
		return
	fi

	TAG=$(fetch "https://api.github.com/repos/$REPO/releases/latest" |
		sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
		head -n 1) || TAG=""
	[ -n "$TAG" ] ||
		die "could not determine the latest release of $REPO, check that the repository has a public release"
}

ensure_install_dir() {
	mkdir -p "$INSTALL_DIR" 2>/dev/null ||
		die "cannot create $INSTALL_DIR, set POBEAR_INSTALL_DIR to a writable directory"
	INSTALL_DIR=$(cd "$INSTALL_DIR" && pwd) ||
		die "cannot resolve $INSTALL_DIR"
	[ -w "$INSTALL_DIR" ] ||
		die "$INSTALL_DIR is not writable, set POBEAR_INSTALL_DIR to a writable directory"
}

check_existing() {
	target="$INSTALL_DIR/$BINARY"
	[ -e "$target" ] || return 0
	REPLACED_EXISTING=1

	[ "${POBEAR_FORCE:-}" = "1" ] && return 0
	[ -x "$target" ] || return 0

	existing=$("$target" version 2>/dev/null) || return 0
	case "$existing" in
	"$BINARY"*) return 0 ;;
	esac
	die "$target exists and is not pobear, set POBEAR_FORCE=1 to overwrite it or choose another POBEAR_INSTALL_DIR"
}

verify_checksum() {
	verify_file="$1"
	verify_name="$2"
	verify_sums="$3"

	expected=$(awk -v name="$verify_name" '$2 == name { print $1 }' "$verify_sums")
	[ -n "$expected" ] || die "$verify_name is not listed in checksums.txt"

	if has sha256sum; then
		actual=$(sha256sum "$verify_file" | awk '{ print $1 }')
	else
		actual=$(shasum -a 256 "$verify_file" | awk '{ print $1 }')
	fi

	[ "$actual" = "$expected" ] || die "checksum mismatch for $verify_name"
}

on_path() {
	case ":$PATH:" in
	*":$1:"*) return 0 ;;
	esac
	return 1
}

shell_rc_file() {
	case "$(basename "${SHELL:-/bin/sh}")" in
	zsh) echo "$HOME/.zshrc" ;;
	bash) echo "$HOME/.bashrc" ;;
	fish) echo "$HOME/.config/fish/config.fish" ;;
	*) echo "$HOME/.profile" ;;
	esac
}

first_writable_path_dir() {
	old_ifs="$IFS"
	IFS=:
	set -f
	set -- $PATH
	set +f
	IFS="$old_ifs"

	for dir in "$@"; do
		case "$dir" in
		/*) ;;
		*) continue ;;
		esac
		[ "$dir" = "$INSTALL_DIR" ] && continue
		if [ -d "$dir" ] && [ -w "$dir" ]; then
			echo "$dir"
			return 0
		fi
	done
	return 1
}

link_onto_path() {
	link_dir=$(first_writable_path_dir) || return 1
	dest="$link_dir/$BINARY"
	if [ -e "$dest" ] && [ ! -L "$dest" ]; then
		return 1
	fi
	ln -sf "$INSTALL_DIR/$BINARY" "$dest" 2>/dev/null || return 1
	LINKED_DIR="$link_dir"
	return 0
}

add_to_rc_file() {
	RC_FILE=$(shell_rc_file)
	if [ -f "$RC_FILE" ] && grep -Fq "$INSTALL_DIR" "$RC_FILE" 2>/dev/null; then
		return 0
	fi
	mkdir -p "$(dirname "$RC_FILE")" 2>/dev/null || return 1
	if [ "$(basename "${SHELL:-/bin/sh}")" = "fish" ]; then
		printf '\nset -gx PATH "%s" $PATH\n' "$INSTALL_DIR" >>"$RC_FILE" || return 1
	else
		printf '\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >>"$RC_FILE" || return 1
	fi
	return 0
}

ensure_reachable() {
	on_path "$INSTALL_DIR" && return 0

	if [ "${POBEAR_NO_MODIFY_PATH:-}" = "1" ]; then
		print_path_hint
		return 0
	fi

	link_onto_path && return 0

	if add_to_rc_file; then
		echo
		echo "Added $INSTALL_DIR to your PATH in $RC_FILE"
		echo "Run this now:  ${BOLD}. $RC_FILE${RESET}"
		echo "Or:            ${BOLD}export PATH=\"$INSTALL_DIR:\$PATH\"${RESET}"
		return 0
	fi

	print_path_hint
}

print_path_hint() {
	echo
	echo "$INSTALL_DIR is not on your PATH. Add it with:"
	echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
}

print_rehash_hint() {
	[ "$REPLACED_EXISTING" = "1" ] || [ -n "$LINKED_DIR" ] || return 0
	case "$(basename "${SHELL:-/bin/sh}")" in
	zsh | bash)
		echo "${DIM}If this shell still runs the old binary, run: hash -r${RESET}"
		;;
	esac
}

verify_installed() {
	installed=$("$INSTALL_DIR/$BINARY" version 2>&1) ||
		die "$INSTALL_DIR/$BINARY was installed but will not run: $installed"
	echo "$installed"
}

main() {
	check_prerequisites
	detect_platform
	ensure_install_dir
	check_existing
	resolve_tag

	archive="${BINARY}_${TAG#v}_${OS}_${ARCH}.tar.gz"
	base_url="https://github.com/$REPO/releases/download/$TAG"

	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	echo "Downloading $BINARY $TAG ($OS/$ARCH)"
	download "$base_url/$archive" "$tmp/$archive" ||
		die "could not download $archive from $TAG, the asset may not exist or the repository may be private"
	download "$base_url/checksums.txt" "$tmp/checksums.txt" ||
		die "could not download checksums.txt from $TAG, the asset may not exist or the repository may be private"

	verify_checksum "$tmp/$archive" "$archive" "$tmp/checksums.txt"

	tar -xzf "$tmp/$archive" -C "$tmp" || die "could not extract $archive"
	[ -f "$tmp/$BINARY" ] || die "$archive does not contain a $BINARY binary"

	mv -f "$tmp/$BINARY" "$INSTALL_DIR/$BINARY.tmp"
	chmod +x "$INSTALL_DIR/$BINARY.tmp"
	mv -f "$INSTALL_DIR/$BINARY.tmp" "$INSTALL_DIR/$BINARY"

	version=$(verify_installed)

	echo "Installed ${BOLD}$version${RESET} to $INSTALL_DIR/$BINARY"
	ensure_reachable
	if [ -n "$LINKED_DIR" ]; then
		echo "Linked into $LINKED_DIR, which is already on your PATH"
	fi
	print_rehash_hint
}

main
