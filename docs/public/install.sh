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
release_url="https://github.com/${repo}/releases/latest/download/${asset}"
# ghfast.top proxies GitHub URLs for networks that cannot reach github.com directly.
mirror_url="https://ghfast.top/${release_url}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

printf 'Downloading %s...\n' "$asset"
if ! curl --fail --location --silent --show-error --connect-timeout 30 \
  "$release_url" --output "$tmp_dir/sshit"; then
  echo "GitHub download failed; retrying through the mirror..." >&2
  rm -f "$tmp_dir/sshit"
  if ! curl --fail --location --silent --show-error --connect-timeout 30 \
    "$mirror_url" --output "$tmp_dir/sshit"; then
    echo "Failed to download ${asset} from both GitHub and the mirror." >&2
    exit 1
  fi
fi
file_type="$(file -bL "$tmp_dir/sshit" 2>/dev/null || file -b "$tmp_dir/sshit" 2>/dev/null || :)"
if [[ -n "$file_type" ]]; then
  case "$file_type" in
    *executable*|*binary*|*Mach-O*|*ELF*) ;;
    *)
      echo "Downloaded file does not look like a binary: $file_type" >&2
      exit 1
      ;;
  esac
fi

chmod +x "$tmp_dir/sshit"

if [[ -w "$install_dir" ]]; then
  install -m 0755 "$tmp_dir/sshit" "$install_dir/sshit"
else
  sudo install -m 0755 -d "$install_dir"
  sudo install -m 0755 "$tmp_dir/sshit" "$install_dir/sshit"
fi

printf 'Installed sshit to %s/sshit\n' "$install_dir"
