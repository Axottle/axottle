#!/usr/bin/env bash

# Download the installer for the advertised release and preserve terminal input
# when this entry point is supplied through curl | bash.
axottle_install() (
  set -euo pipefail
  readonly version='v2.0.0-beta.1'
  readonly installer_sha256='e28ee1de0235fb18e1930c2962741c9e1f9e3de7a8e6eda44be7e29ad2dbfd5f'
  readonly installer_revision='r2'
  readonly ui_amd64_sha256='c5d64aefe839584118ecf48b3d5b949f1803fe3cf2e0beef7f28c246fcc28a2a'
  readonly ui_arm64_sha256='6c54c3b73f45b1d3ef2edffabcd943d30f429776abe894ef662938534d306fb6'
  local work_dir
  # sudo's PTY must receive terminal input, not the bootstrap script pipe.
  local parent_command=''
  if [[ ! -t 0 && -r /proc/$PPID/comm ]]; then
    read -r parent_command < "/proc/$PPID/comm" || true
    case "$parent_command" in
      sudo|sudo-rs)
        printf '%s\n' 'Run the bootstrap without sudo; it requests privileges after downloading:' \
          'curl -fsSL https://raw.githubusercontent.com/Axottle/axottle/main/install.sh | bash' >&2
        exit 1
        ;;
    esac
  fi
  local -a elevate=()
  if (( EUID != 0 )); then
    command -v sudo >/dev/null || { echo 'Run as root or install sudo before continuing.' >&2; exit 1; }
    elevate=(sudo)
  fi
  command -v curl >/dev/null || { echo 'Install curl before continuing.' >&2; exit 1; }
  printf 'Axottle %s: downloading installer…\n' "$version" >&2
  work_dir="$(mktemp -d)"
  trap 'rm -rf -- "$work_dir"' EXIT

  fetch_checked() {
    local asset="$1" destination="$2" expected="$3" checksum
    curl --fail --silent --show-error --location \
      --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 120 \
      "https://github.com/Axottle/axottle/releases/download/$version/$asset" \
      --output "$destination" || {
        echo "Cannot download $asset. Check your connection and release availability." >&2
        exit 1
      }
    if command -v sha256sum >/dev/null; then
      checksum="$(sha256sum "$destination")"
    elif command -v shasum >/dev/null; then
      checksum="$(shasum -a 256 "$destination")"
    else
      echo 'Install sha256sum or shasum before continuing.' >&2
      exit 1
    fi
    [[ "${checksum%% *}" == "$expected" ]] || {
      echo 'Installer checksum mismatch; nothing was executed.' >&2
      exit 1
    }
  }
  fetch_checked "axottle-installer-$installer_revision.sh" "$work_dir/installer.sh" "$installer_sha256"

  local arch ui_sha use_ui=true argument
  for argument in "$@"; do
    case "$argument" in --help|-h) use_ui=false ;; esac
  done
  if ! ( : </dev/tty ) 2>/dev/null; then use_ui=false; fi
  if [[ "$use_ui" == true ]]; then
    case "$(uname -m)" in
      x86_64|amd64) arch=amd64; ui_sha="$ui_amd64_sha256" ;;
      aarch64|arm64) arch=arm64; ui_sha="$ui_arm64_sha256" ;;
      *) echo 'Supported server architectures: amd64, arm64.' >&2; exit 1 ;;
    esac
    fetch_checked "axottle-installer-ui-$installer_revision-linux-$arch" "$work_dir/installer-ui" "$ui_sha"
    chmod 700 "$work_dir/installer-ui"
    printf '%s\n' 'Installer verified. Starting setup…' >&2
    # The frontend writes to /dev/tty itself. A non-terminal stdout prevents
    # eager library theme queries from delaying startup or consuming input.
    "${elevate[@]}" "$work_dir/installer-ui" "$work_dir/installer.sh" --repo Axottle/axottle --release "$version" "$@" </dev/tty >/dev/null
  else
    printf '%s\n' 'Installer verified. Starting setup…' >&2
    "${elevate[@]}" bash "$work_dir/installer.sh" --repo Axottle/axottle --release "$version" "$@" </dev/null
  fi
)

axottle_install "$@"
