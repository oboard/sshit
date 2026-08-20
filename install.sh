#!/usr/bin/env bash
set -euo pipefail

repo="oboard/sshit"
install_dir="${INSTALL_DIR:-/usr/local/bin}"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="macos" ;;
  *)
    echo "Unsupported operating system: $(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="x64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    echo "Unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

asset="sshit-${os}-${arch}"
url="https://github.com/${repo}/releases/latest/download/${asset}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

printf 'Downloading %s...\n' "$asset"
curl --fail --location --silent --show-error "$url" --output "$tmp_dir/sshit"
chmod +x "$tmp_dir/sshit"

if [[ -w "$install_dir" ]]; then
  install -m 0755 "$tmp_dir/sshit" "$install_dir/sshit"
else
  sudo install -m 0755 "$tmp_dir/sshit" "$install_dir/sshit"
fi

printf 'Installed sshit to %s/sshit\n' "$install_dir"
