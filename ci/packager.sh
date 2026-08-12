#!/usr/bin/env bash
set -euo pipefail

osarch=(
  darwin/amd64
  darwin/arm64
  linux/amd64
  linux/arm
  linux/arm64
  windows/amd64
)

function log() {
  echo "[$(date +%H:%M:%S)] $*" >&2
}

function compress_archive() {
  local target_os="$1"
  local archive_base="$2"
  local binary_name="$3"

  if command -v tar &> /dev/null; then
    tar -czf "${archive_base}.tgz" "${binary_name}"
  elif command -v zip &> /dev/null; then
    zip -q "${archive_base}.zip" "${binary_name}"
  elif command -v powershell &> /dev/null; then
    powershell -NoProfile -Command "Compress-Archive -Path '${binary_name}' -DestinationPath '${archive_base}.zip' -Force"
  else
    log "ERR: No compression utility (tar/zip/powershell) available for ${archive_base}"
    return 1
  fi
}

function go_package() {
  local os="$1"
  local arch="$2"
  local pkg_name="$3"
  local pkg_dir="$4"

  local outname="${pkg_name}"
  [[ $os == windows ]] && outname="${pkg_name}.exe"

  log "=> Building ${pkg_name} for ${os}/${arch}..."
  (
    cd "${pkg_dir}"
    CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build \
      -buildvcs=false \
      -ldflags "-s -w -buildid= -X main.version=${version}" \
      -mod=readonly \
      -trimpath \
      -o "${outname}"

    log "=> Packaging ${pkg_name} as ${pkg_name}_${os}_${arch}..."
    compress_archive "${os}" "${builddir}/${pkg_name}_${os}_${arch}" "${outname}"
    rm -f "${outname}"
  )
}

function go_package_all() {
  for oa in "${osarch[@]}"; do
    local os arch
    os=$(cut -d / -f 1 <<< "${oa}")
    arch=$(cut -d / -f 2 <<< "${oa}")
    go_package "${os}" "${arch}" "${1}" "${2}"
  done
}

root="$(pwd)"
builddir="${root}/testfiles/dist"
version="${OTS_VERSION:-$(git describe --tags --always 2>/dev/null || echo dev)}"

log "Building version ${version} into ${builddir}..."

log "Resetting output directory..."
rm -rf "${builddir}"
mkdir -p "${builddir}"

log "Building API-Server..."
go_package_all "ots" "."

log "Building OTS-CLI..."
go_package_all "ots-cli" "./cmd/ots-cli"

log "Generating SHA256SUMS file..."
(
  cd "${builddir}"
  if command -v sha256sum &> /dev/null; then
    sha256sum ./* > SHA256SUMS
  elif command -v shasum &> /dev/null; then
    shasum -a 256 ./* > SHA256SUMS
  fi
)

log "Packaging complete. Archives saved to ${builddir}"
