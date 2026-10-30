#!/usr/bin/env bash

# Download the installer for the advertised release and preserve terminal input
# when this entry point is supplied through curl | bash.
axottle_install() (
  set -euo pipefail
  readonly version='v2.0.0-beta.1'
  readonly installer_sha256='55e4db474697286b7b3424bd17a00efaf42ecada62086645943d72447013fa46'
  local work_dir
  command -v curl >/dev/null || { echo 'Install curl before continuing.' >&2; exit 1; }
  work_dir="$(mktemp -d)"
  trap 'rm -rf -- "$work_dir"' EXIT

  if ! curl --fail --silent --show-error --location \
    --proto '=https' --proto-redir '=https' --connect-timeout 15 --max-time 120 \
    "https://github.com/Axottle/axottle/releases/download/$version/axottle-installer.sh" \
    --output "$work_dir/installer.sh"; then
    echo 'Installer unavailable. Both Axottle release repositories must be public for installation without a token.' >&2
    exit 1
  fi

  local checksum
  if command -v sha256sum >/dev/null; then
    checksum="$(sha256sum "$work_dir/installer.sh")"
  elif command -v shasum >/dev/null; then
    checksum="$(shasum -a 256 "$work_dir/installer.sh")"
  else
    echo 'Install sha256sum or shasum before continuing.' >&2
    exit 1
  fi
  [[ "${checksum%% *}" == "$installer_sha256" ]] || {
    echo 'Installer checksum mismatch; nothing was executed.' >&2
    exit 1
  }

  if ( : </dev/tty ) 2>/dev/null; then
    bash "$work_dir/installer.sh" --repo Axottle/axottle --release "$version" "$@" </dev/tty
  else
    bash "$work_dir/installer.sh" --repo Axottle/axottle --release "$version" "$@" </dev/null
  fi
)

axottle_install "$@"
