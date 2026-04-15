#!/usr/bin/env bash
# install_local.sh — Build imghost + imghostd from the current source tree and
# install them to ~/.local/bin, then (re)start the imghostd user service.
#
# Usage:
#   bash scripts/install_local.sh
#
# Environment variables:
#   GO_BIN=go                     # override the `go` binary
#   IMGHOST_NO_MODIFY_PATH=1      # do not write PATH to shell profile
#
# Prerequisite: Go toolchain installed and reachable via ${GO_BIN}.
#
# This is the source-build counterpart to the release-binary install.sh at the
# repo root. Keep the two scripts self-contained (no shared lib) so install.sh
# stays curl|bash-safe as a single file.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
INSTALL_DIR="${HOME}/.local/bin"
LAUNCHD_LABEL="com.imghost.imghostd"
GO_BIN="${GO_BIN:-go}"

check_go() {
  if ! command -v "${GO_BIN}" >/dev/null 2>&1; then
    echo "Error: ${GO_BIN} is required to build from source." >&2
    exit 1
  fi
}

detect_os() {
  OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
  if [[ "${OS}" != "linux" && "${OS}" != "darwin" ]]; then
    echo "Error: unsupported OS for local install: ${OS}" >&2
    exit 1
  fi
}

build_binaries() {
  mkdir -p "${INSTALL_DIR}"
  cd "${PROJECT_ROOT}"

  echo "Building imghost and imghostd from ${PROJECT_ROOT}..."
  "${GO_BIN}" build -ldflags="-s -w" -o "${INSTALL_DIR}/imghost"  ./cmd/imghost
  "${GO_BIN}" build -ldflags="-s -w" -o "${INSTALL_DIR}/imghostd" ./cmd/imghostd

  echo "Installed: ${INSTALL_DIR}/imghost"
  echo "Installed: ${INSTALL_DIR}/imghostd"
}

ensure_path() {
  if [[ "${IMGHOST_NO_MODIFY_PATH:-0}" == "1" ]]; then
    return
  fi

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
      if [[ "${OS}" == "darwin" ]]; then
        profile="${HOME}/.bash_profile"
      else
        profile="${HOME}/.bashrc"
      fi
      ;;
    *)    profile="${HOME}/.profile" ;;
  esac

  local marker_begin="# >>> imghost installer >>>"
  local marker_end="# <<< imghost installer <<<"

  if [[ -f "${profile}" ]] && grep -qF "${marker_begin}" "${profile}"; then
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

enable_linger() {
  if ! loginctl enable-linger "${USER}" 2>/dev/null; then
    echo "Warning: loginctl enable-linger failed; the daemon may stop when you log out." >&2
    echo "         You can retry later with: sudo loginctl enable-linger ${USER}" >&2
  fi
}

setup_systemd() {
  local service_dir="${HOME}/.config/systemd/user"
  local service_file="${service_dir}/imghostd.service"

  mkdir -p "${service_dir}"

  cat > "${service_file}.tmp" <<EOF
[Unit]
Description=imghost daemon

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/imghostd
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
  systemctl --user enable imghostd
  systemctl --user restart imghostd
}

setup_launchd() {
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
  <key>ProgramArguments</key><array><string>${INSTALL_DIR}/imghostd</string></array>
  <key>RunAtLoad</key>      <true/>
  <key>KeepAlive</key>
    <dict><key>SuccessfulExit</key><false/></dict>
  <key>StandardOutPath</key><string>${HOME}/Library/Logs/imghostd.log</string>
  <key>StandardErrorPath</key><string>${HOME}/Library/Logs/imghostd.log</string>
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
  echo "Local install complete."
  echo "  CLI    : ${INSTALL_DIR}/imghost"
  echo "  daemon : ${INSTALL_DIR}/imghostd"
  case "${OS}" in
    linux)  echo "Service: systemctl --user status imghostd" ;;
    darwin) echo "Service: launchctl list | grep imghostd" ;;
  esac
  echo "Swagger UI: http://localhost:34286/swagger/index.html"
}

main() {
  check_go
  detect_os
  build_binaries
  ensure_path
  case "${OS}" in
    linux)  enable_linger; setup_systemd ;;
    darwin) setup_launchd ;;
  esac
  print_summary
}

main "$@"
