#!/bin/sh

set -eu

REPO="${GITHUB_REPOSITORY:-mishamyrt/rakun}"
BINARY_NAME="rakun"
VERSION="${RAKUN_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-}"

log() {
  printf '%s\n' "$*" >&2
}

fail() {
  log "Error: $*"
  exit 1
}

has_cmd() {
  command -v "$1" >/dev/null 2>&1
}

need_cmd() {
  has_cmd "$1" || fail "Required command not found: $1"
}

http_get() {
  url=$1

  if has_cmd curl; then
    curl -fsSL "$url"
    return
  fi

  if has_cmd wget; then
    wget -qO- "$url"
    return
  fi

  fail "curl or wget is required"
}

download_file() {
  url=$1
  destination=$2

  if has_cmd curl; then
    curl -fsSL "$url" -o "$destination" || fail "Failed to download $url"
    return
  fi

  if has_cmd wget; then
    wget -qO "$destination" "$url" || fail "Failed to download $url"
    return
  fi

  fail "curl or wget is required"
}

detect_os() {
  case "$(uname -s)" in
    Darwin)
      printf '%s\n' "darwin"
      ;;
    Linux)
      printf '%s\n' "linux"
      ;;
    *)
      fail "Unsupported operating system: $(uname -s)"
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      printf '%s\n' "amd64"
      ;;
    arm64|aarch64)
      printf '%s\n' "arm64"
      ;;
    *)
      fail "Unsupported architecture: $(uname -m)"
      ;;
  esac
}

normalize_tag() {
  case "$1" in
    v*)
      printf '%s\n' "$1"
      ;;
    *)
      printf 'v%s\n' "$1"
      ;;
  esac
}

resolve_tag() {
  case "$VERSION" in
    ""|latest)
      tag=$(
        http_get "https://api.github.com/repos/${REPO}/releases/latest" |
          awk -F'"' '/"tag_name"[[:space:]]*:/ { print $4; exit }'
      )
      [ -n "$tag" ] || fail "Could not determine the latest release for ${REPO}"
      printf '%s\n' "$tag"
      ;;
    *)
      normalize_tag "$VERSION"
      ;;
  esac
}

choose_install_dir() {
  if [ -n "$INSTALL_DIR" ]; then
    printf '%s\n' "$INSTALL_DIR"
    return
  fi

  if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
    printf '%s\n' "/usr/local/bin"
    return
  fi

  printf '%s\n' "${HOME}/.local/bin"
}

ensure_writable_dir() {
  dir=$1

  if [ ! -d "$dir" ]; then
    mkdir -p "$dir" || fail "Could not create install directory: $dir"
  fi

  [ -w "$dir" ] || fail "Install directory is not writable: $dir"
}

verify_checksum() {
  checksum_file=$1
  archive_path=$2
  asset_name=$3

  expected_checksum=$(
    awk -v name="$asset_name" '$2 == name { print $1; exit }' "$checksum_file"
  )
  [ -n "$expected_checksum" ] || fail "Checksum for ${asset_name} was not found"

  if has_cmd sha256sum; then
    actual_checksum=$(sha256sum "$archive_path" | awk '{ print $1 }')
  elif has_cmd shasum; then
    actual_checksum=$(shasum -a 256 "$archive_path" | awk '{ print $1 }')
  else
    fail "sha256sum or shasum is required for checksum verification"
  fi

  [ "$expected_checksum" = "$actual_checksum" ] || fail "Checksum mismatch for ${asset_name}"
}

need_cmd awk
need_cmd tar
need_cmd mktemp

os=$(detect_os)
arch=$(detect_arch)
tag=$(resolve_tag)
asset_name="${BINARY_NAME}_${tag}_${os}_${arch}.tar.gz"
release_url="https://github.com/${REPO}/releases/download/${tag}"
archive_url="${release_url}/${asset_name}"
checksum_url="${release_url}/sha256sums.txt"
target_dir=$(choose_install_dir)
target_path="${target_dir}/${BINARY_NAME}"

tmpdir=$(mktemp -d "${TMPDIR:-/tmp}/rakun-install.XXXXXX") || fail "Could not create temporary directory"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT INT TERM

log "Installing ${BINARY_NAME} ${tag} for ${os}/${arch}"
download_file "$archive_url" "${tmpdir}/${asset_name}"
download_file "$checksum_url" "${tmpdir}/sha256sums.txt"
verify_checksum "${tmpdir}/sha256sums.txt" "${tmpdir}/${asset_name}" "$asset_name"

tar -xzf "${tmpdir}/${asset_name}" -C "$tmpdir" || fail "Failed to extract ${asset_name}"
[ -f "${tmpdir}/${BINARY_NAME}" ] || fail "Archive ${asset_name} does not contain ${BINARY_NAME}"

ensure_writable_dir "$target_dir"

if has_cmd install; then
  install -m 0755 "${tmpdir}/${BINARY_NAME}" "$target_path"
else
  cp "${tmpdir}/${BINARY_NAME}" "$target_path"
  chmod 0755 "$target_path"
fi

log "Installed ${BINARY_NAME} to ${target_path}"

case ":${PATH}:" in
  *:"${target_dir}":*)
    ;;
  *)
    log "Note: ${target_dir} is not in PATH. Add it to your shell profile to run ${BINARY_NAME} directly."
    ;;
esac

"$target_path" --version
