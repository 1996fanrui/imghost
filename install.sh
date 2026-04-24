#!/usr/bin/env bash
# Install or upgrade filehub (filehub CLI + filehubd daemon) from GitHub Releases.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/1996fanrui/filehub/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/1996fanrui/filehub/main/install.sh | bash -s -- v0.1.1
#   curl -fsSL https://raw.githubusercontent.com/1996fanrui/filehub/main/install.sh | bash -s -- --pre
#
# Examples:
#   bash install.sh                   # install latest stable release
#   bash install.sh v0.1.1            # install specific stable version
#   bash install.sh v0.1.1-alpha.3    # install specific pre-release
#   bash install.sh --pre             # install latest release (including pre-releases)
#
# Environment variables:
#   FILEHUB_NO_MODIFY_PATH=1          # do not write PATH to shell profile
#
# What this script does:
#   1. Resolves the target version from the GitHub Releases API.
#   2. Downloads filehub and filehubd binaries for your OS/arch to ~/.local/bin.
#   3. Ensures ~/.local/bin is on PATH via a marker block in your shell profile.
#   4. Linux : writes ~/.config/systemd/user/filehubd.service, enables linger,
#              and runs `systemctl --user enable --now filehubd`.
#   5. macOS : writes ~/Library/LaunchAgents/com.filehub.filehubd.plist and
#              bootstraps it into the gui/$UID domain.

set -e

# Constants (read-only; safe at source-time).
GITHUB_REPO="1996fanrui/filehub"
INSTALL_DIR="${HOME}/.local/bin"
LAUNCHD_LABEL="com.filehub.filehubd"

# ---------------------------------------------------------------------------
# Helper functions (pure; no side effects at source-time).
# ---------------------------------------------------------------------------

reject_windows_shell() {
  case "${OSTYPE:-}" in
    msys*|mingw*|cygwin*)
      echo "Error: this script does not support Windows-like shells (${OSTYPE})." >&2
      echo "" >&2
      echo "Please use install.ps1 from a PowerShell prompt instead:" >&2
      echo "" >&2
      echo "  iwr https://raw.githubusercontent.com/${GITHUB_REPO}/main/install.ps1 -UseBasicParsing | iex" >&2
      exit 1
      ;;
  esac
}

check_curl() {
  if ! command -v curl >/dev/null 2>&1; then
    echo "Error: curl is required but not found." >&2
    exit 1
  fi
}

detect_os_arch() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  case "${ARCH}" in
    x86_64)        ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
      echo "Error: unsupported architecture: ${ARCH}" >&2
      exit 1
      ;;
  esac

  if [[ "${OS}" != "linux" && "${OS}" != "darwin" ]]; then
    echo "Error: unsupported OS: ${OS}" >&2
    exit 1
  fi
}

resolve_version() {
  # VERSION may already be set by parse_args (exact tag). Otherwise hit API.
  if [[ -n "${VERSION}" ]]; then
    return
  fi

  local api_response
  if [[ "${INCLUDE_PRE}" == "true" ]]; then
    echo "Fetching latest release version (including pre-releases)..."
    api_response=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases")
    VERSION=$(echo "${api_response}" | grep -m1 '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  else
    echo "Fetching latest stable release version..."
    api_response=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest")
    VERSION=$(echo "${api_response}" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  fi

  if [[ -z "${VERSION}" ]]; then
    echo "Error: could not determine latest version from GitHub API." >&2
    exit 1
  fi
}

download_binaries() {
  local want_version="${VERSION#v}"
  local need_download=true

  if [[ -x "${INSTALL_DIR}/filehub" ]]; then
    local current_version
    current_version=$("${INSTALL_DIR}/filehub" version 2>/dev/null | awk 'NR==1 {print $3}' || true)
    if [[ -n "${current_version}" && "${current_version}" == "${want_version}" ]]; then
      need_download=false
      echo ""
      echo "Already at version ${want_version}, skipping download."
    fi
  fi

  if [[ "${need_download}" != "true" ]]; then
    return
  fi

  local suffix="${OS}_${ARCH}"
  local base_url="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}"
  local tmp_dir
  tmp_dir=$(mktemp -d)
  trap 'rm -rf "${tmp_dir}"' EXIT

  echo ""
  echo "Downloading filehub_${suffix}..."
  curl -fSL "${base_url}/filehub_${suffix}" -o "${tmp_dir}/filehub_${suffix}"

  echo "Downloading filehubd_${suffix}..."
  curl -fSL "${base_url}/filehubd_${suffix}" -o "${tmp_dir}/filehubd_${suffix}"

  mkdir -p "${INSTALL_DIR}"
  install -m 755 "${tmp_dir}/filehub_${suffix}"  "${INSTALL_DIR}/filehub"
  install -m 755 "${tmp_dir}/filehubd_${suffix}" "${INSTALL_DIR}/filehubd"

  echo ""
  echo "Installed: ${INSTALL_DIR}/filehub"
  echo "Installed: ${INSTALL_DIR}/filehubd"
}

ensure_path() {
  # Respect opt-out.
  if [[ "${FILEHUB_NO_MODIFY_PATH:-0}" == "1" ]]; then
    return
  fi

  # Already on PATH — still ensure current process has it, then skip profile write.
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*)
      return
      ;;
  esac

  local shell_name
  shell_name=$(basename "${SHELL:-/bin/bash}")

  local profile=""
  case "${shell_name}" in
    zsh)  profile="${HOME}/.zshrc" ;;
    bash)
      if [[ "$(uname -s)" == "Darwin" ]]; then
        profile="${HOME}/.bash_profile"
      else
        profile="${HOME}/.bashrc"
      fi
      ;;
    *)    profile="${HOME}/.profile" ;;
  esac

  local marker_begin="# >>> filehub installer >>>"
  local marker_end="# <<< filehub installer <<<"

  if [[ -f "${profile}" ]] && grep -qF "${marker_begin}" "${profile}"; then
    # Already installed the marker block; nothing to do.
    export PATH="${INSTALL_DIR}:${PATH}"
    return
  fi

  {
    echo ""
    echo "${marker_begin}"
    echo "export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo "${marker_end}"
  } >> "${profile}"
  echo "Added ${INSTALL_DIR} to PATH in ${profile}"

  export PATH="${INSTALL_DIR}:${PATH}"
}

setup_systemd() {
  local filehubd_bin="${INSTALL_DIR}/filehubd"
  local service_dir="${HOME}/.config/systemd/user"
  local service_file="${service_dir}/filehubd.service"

  mkdir -p "${service_dir}"

  cat > "${service_file}.tmp" <<EOF
[Unit]
Description=filehub daemon

[Service]
Type=simple
ExecStart=${filehubd_bin}
SuccessExitStatus=143
Restart=on-failure
RestartSec=5
TimeoutStopSec=30
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
EOF

  mv -f "${service_file}.tmp" "${service_file}"

  systemctl --user daemon-reload
  systemctl --user enable --now filehubd
}

enable_linger() {
  if ! loginctl enable-linger "${USER}" 2>/dev/null; then
    echo "Warning: loginctl enable-linger failed; the daemon may stop when you log out." >&2
    echo "         You can retry later with: sudo loginctl enable-linger ${USER}" >&2
  fi
}

setup_launchd() {
  local filehubd_bin="${INSTALL_DIR}/filehubd"
  local agents_dir="${HOME}/Library/LaunchAgents"
  local plist_file="${agents_dir}/${LAUNCHD_LABEL}.plist"

  mkdir -p "${agents_dir}"
  mkdir -p "${HOME}/Library/Logs"

  cat > "${plist_file}" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>          <string>${LAUNCHD_LABEL}</string>
  <key>ProgramArguments</key><array><string>${filehubd_bin}</string></array>
  <key>RunAtLoad</key>      <true/>
  <key>KeepAlive</key>
    <dict><key>SuccessfulExit</key><false/></dict>
  <key>StandardOutPath</key><string>${HOME}/Library/Logs/filehubd.log</string>
  <key>StandardErrorPath</key><string>${HOME}/Library/Logs/filehubd.log</string>
</dict>
</plist>
EOF

  local domain target
  domain="gui/$(id -u)"
  target="${domain}/${LAUNCHD_LABEL}"

  if launchctl print "${target}" >/dev/null 2>&1; then
    launchctl bootout "${target}"
  fi
  launchctl bootstrap "${domain}" "${plist_file}"
}

print_summary() {
  echo ""
  echo "Installation complete. filehub ${VERSION} is ready."
  echo "  CLI    : ${INSTALL_DIR}/filehub"
  echo "  daemon : ${INSTALL_DIR}/filehubd"
  echo ""
  case "$(uname -s)" in
    Linux)  echo "Linux:  systemctl --user status filehubd" ;;
    Darwin) echo "macOS:  launchctl list | grep filehubd" ;;
  esac
  echo "Swagger UI: http://localhost:34286/swagger/index.html"
}

parse_args() {
  VERSION=""
  INCLUDE_PRE=false

  for arg in "$@"; do
    case "${arg}" in
      --pre)     INCLUDE_PRE=true ;;
      v*)        VERSION="${arg}" ;;
      "")        ;;
      *)
        echo "Error: unrecognized argument: ${arg}" >&2
        echo "Usage: install.sh [--pre] [vX.Y.Z[-alpha.N]]" >&2
        exit 1
        ;;
    esac
  done
}

# ---------------------------------------------------------------------------
# main: orchestrates the full install. All logic lives here so a truncated
# download (from curl|bash) cannot execute a partial script — only the final
# `main "$@"` call below triggers side effects.
# ---------------------------------------------------------------------------
main() {
  parse_args "$@"
  reject_windows_shell
  check_curl
  detect_os_arch
  resolve_version

  echo "Version : ${VERSION}"
  echo "OS/Arch : ${OS}/${ARCH}"
  echo "Install : ${INSTALL_DIR}"

  download_binaries
  ensure_path

  case "$(uname -s)" in
    Linux)  enable_linger; setup_systemd ;;
    Darwin) setup_launchd ;;
  esac

  print_summary
}

main "$@"
