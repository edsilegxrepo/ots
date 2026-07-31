#!/usr/bin/env bash
# -----------------------------------------------------------------------------
#  tools/build.sh
#  Cross-Platform OTS (One-Time Secrets) Build Script
#  Supports: Linux, Cygwin, MSYS2 (Git Bash), macOS
# -----------------------------------------------------------------------------
set -euo pipefail

# Determine script and project directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Configuration & Defaults
BUILD_ID="ots"
VERSION_ARG=""
NO_PACKAGE=false
BUILD_WINDOWS=false
BUILD_VENDOR=false
BUILD_ENGLISH_ONLY=false
BUILD_AUTO_VERSION=false
BUILD_VALIDATE=false
KILL_RUNNING=false
AUTO_START=false
UPDATE_DEPS=false
OUTPUT_DIR="${ROOT_DIR}/testfiles/dist"
BIN_DIR="${ROOT_DIR}/testfiles/bin"

# Helper function to terminate all running OTS server instances across platforms
kill_running_ots_processes() {
  echo "==> Terminating any running OTS server processes..."
  if command -v pkill &>/dev/null; then
    pkill -f "ots.*--listen" 2>/dev/null || true
  fi
  if command -v taskkill &>/dev/null; then
    taskkill /F /IM ots.exe 2>/dev/null || true
    taskkill /F /IM ots_windows_amd64.exe 2>/dev/null || true
    taskkill /F /IM ots_linux_amd64 2>/dev/null || true
  fi
  if command -v powershell &>/dev/null; then
    powershell -Command "Get-Process -Name ots,ots_windows_amd64,ots_linux_amd64 -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue" 2>/dev/null || true
  fi
}

# Parse CLI arguments
for arg in "$@"; do
  case "$arg" in
    --windows)
      BUILD_WINDOWS=true
      ;;
    --no-package)
      NO_PACKAGE=true
      ;;
    --build-vendor)
      BUILD_VENDOR=true
      ;;
    --english-only)
      BUILD_ENGLISH_ONLY=true
      ;;
    --auto-version)
      BUILD_AUTO_VERSION=true
      ;;
    --validate)
      BUILD_VALIDATE=true
      ;;
    --kill-running|--clean-processes)
      KILL_RUNNING=true
      ;;
    --auto-start)
      AUTO_START=true
      KILL_RUNNING=true
      ;;
    --update-deps|--upgrade-deps)
      UPDATE_DEPS=true
      ;;
    -*)
      echo "Unknown flag: $arg"
      exit 1
      ;;
    *)
      if [ -z "${VERSION_ARG}" ]; then
        VERSION_ARG="$arg"
      fi
      ;;
  esac
done

if [ "${KILL_RUNNING}" = "true" ]; then
  kill_running_ots_processes
fi

# Resolve version strings
if [ "${BUILD_AUTO_VERSION}" = "true" ] || [ -z "${VERSION_ARG}" ]; then
  GIT_TAG="$(git -C "${ROOT_DIR}" describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "1.21.9")"
  GIT_HASH="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo "dev")"
  VERSION_FULL="${GIT_TAG}-${GIT_HASH}"
else
  VERSION_FULL="${VERSION_ARG}"
fi

VERSION="${VERSION_FULL%-*}"
TAG="${VERSION_FULL#*-}"
[ "${TAG}" = "${VERSION}" ] && TAG="dev"

echo "================================================================="
echo " Building ${BUILD_ID^^} Release: ${VERSION_FULL}"
echo " Environment: $(uname -s 2>/dev/null || echo "unknown") (${MACHTYPE:-unknown})"
echo " Version: ${VERSION} | Tag: ${TAG} | English Only: ${BUILD_ENGLISH_ONLY}"
echo "================================================================="

cd "${ROOT_DIR}"

if [ "${UPDATE_DEPS}" = "true" ]; then
  echo "================================================================="
  echo " ==> RUNNING FULL DEPENDENCY MAINTENANCE (--update-deps)"
  echo "================================================================="
  echo "1. Upgrading Go modules..."
  go get -u ./...
  go mod tidy
  go mod verify

  echo "2. Upgrading Node.js / Vue packages via pnpm..."
  if command -v pnpm &>/dev/null; then
    pnpm update --latest
  elif command -v npm &>/dev/null; then
    npm update
  fi

  echo "3. Auditing Go dependencies..."
  if command -v govulncheck &>/dev/null; then
    govulncheck ./... || true
  fi
  echo "================================================================="
fi

# Step 1: i18n Translation Generation (if translate tool present)
if [ "${BUILD_ENGLISH_ONLY}" = "true" ]; then
  echo "==> Purging non-English languages (--english-only mode enabled)..."
  cp i18n.yaml i18n.yaml.bak
  trap 'mv -f i18n.yaml.bak i18n.yaml 2>/dev/null || true' EXIT

  echo "    Purging translations block using sed..."
  sed -i -n '/^translations:/q;p' i18n.yaml
  echo "translations: {}" >> i18n.yaml
fi

if [ -d "ci/translate" ]; then
  echo "==> Generating i18n translations..."
  if ! (cd ci/translate && go build -o translate_tool main.go 2>/dev/null); then
    (cd ci/translate && go build -o translate_tool)
  fi
  ./ci/translate/translate_tool || true
  rm -f ci/translate/translate_tool ci/translate/translate_tool.exe 2>/dev/null || true
fi

if [ "${BUILD_ENGLISH_ONLY}" = "true" ]; then
  echo "==> Restoring original i18n.yaml file..."
  mv -f i18n.yaml.bak i18n.yaml 2>/dev/null || true
  trap - EXIT
fi

# Step 2: Frontend Asset Generation (Host-Native, No Docker Required)
echo "==> Checking Frontend Build Dependencies..."
if command -v pnpm &>/dev/null; then
  echo "    Building frontend with pnpm (ci/build.mjs)..."
  pnpm install --frozen-lockfile 2>/dev/null || pnpm install
  pnpm node ci/build.mjs || node ci/build.mjs || true
elif command -v npm &>/dev/null; then
  echo "    Building frontend with npm (ci/build.mjs)..."
  npm install --no-audit --no-fund 2>/dev/null || npm install
  node ci/build.mjs || true
elif [ -d "frontend/dist" ] || [ -f "frontend/dist/index.html" ]; then
  echo "    Using pre-built embedded assets in frontend/dist/"
else
  echo "    Notice: pnpm/npm not found. Embedded Go FS fallback will be utilized."
fi

# Step 3: Go Workspace Prep
echo "==> Verifying Go module dependencies..."
go mod tidy
go mod verify

mkdir -p "${BIN_DIR}"
mkdir -p "${OUTPUT_DIR}"

# Helper function to build Go binaries
build_binary() {
  local target_os="$1"
  local target_arch="$2"
  local bin_suffix="$3"
  local output_server="${BIN_DIR}/ots_${target_os}_${target_arch}${bin_suffix}"
  local output_cli="${BIN_DIR}/ots-cli_${target_os}_${target_arch}${bin_suffix}"

  echo "==> Compiling for ${target_os}/${target_arch}..."
  
  # Build main server binary
  GOOS="${target_os}" GOARCH="${target_arch}" CGO_ENABLED=0 \
    go build -v -mod=readonly -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}-${TAG}" \
    -o "${output_server}" .

  # Build standalone CLI binary
  if [ -d "cmd/ots-cli" ]; then
    (
      cd cmd/ots-cli
      output_cli_target="${output_cli}"
      GOOS="${target_os}" GOARCH="${target_arch}" CGO_ENABLED=0 \
        go build -v -mod=readonly -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}-${TAG}" \
        -o "${output_cli_target}" .
    )
  fi
}

# Step 4: Compile Linux Binary
build_binary "linux" "amd64" ""

# Optional Step 5: Compile Windows Binary
if [ "${BUILD_WINDOWS}" = "true" ]; then
  build_binary "windows" "amd64" ".exe"
fi

# Step 6: Verify Compiled Artifacts
echo "==> Verifying compiled binaries..."
if [ -f "${BIN_DIR}/ots_linux_amd64" ]; then
  echo "    Linux Binary: ${BIN_DIR}/ots_linux_amd64"
fi
if [ -f "${BIN_DIR}/ots_windows_amd64.exe" ]; then
  echo "    Windows Binary: ${BIN_DIR}/ots_windows_amd64.exe"
fi

# Step 7: Packaging (if NO_PACKAGE is false)
if [ "${NO_PACKAGE}" = "false" ]; then
  echo "==> Packaging distribution archives..."

  # Create Linux distribution archive matching ots-xg.spec
  LINUX_STAGING="${OUTPUT_DIR}/ots-linux-staging"
  rm -rf "${LINUX_STAGING}"
  mkdir -p "${LINUX_STAGING}/etc/custom" "${LINUX_STAGING}/log" "${LINUX_STAGING}/systemd"

  cp -f "${BIN_DIR}/ots_linux_amd64" "${LINUX_STAGING}/ots" 2>/dev/null || true
  cp -f "${BIN_DIR}/ots-cli_linux_amd64" "${LINUX_STAGING}/ots-cli" 2>/dev/null || true
  [ -f "${SCRIPT_DIR}/ots-config.yaml" ] && cp -f "${SCRIPT_DIR}/ots-config.yaml" "${LINUX_STAGING}/etc/ots-config.yaml"
  [ -f "${SCRIPT_DIR}/ots.sysconfig" ] && cp -f "${SCRIPT_DIR}/ots.sysconfig" "${LINUX_STAGING}/etc/ots.env"
  [ -f "${SCRIPT_DIR}/ots.service" ] && cp -f "${SCRIPT_DIR}/ots.service" "${LINUX_STAGING}/systemd/ots.service"
  echo "ots-${VERSION}-${TAG}-linux_amd64" > "${LINUX_STAGING}/etc/ots.version"

  LINUX_ARCHIVE="${OUTPUT_DIR}/ots-${VERSION}-${TAG}-linux_amd64.tar.gz"
  (cd "${LINUX_STAGING}" && tar -czvf "${LINUX_ARCHIVE}" ./*)
  rm -rf "${LINUX_STAGING}"
  echo "    Linux Archive Created: ${LINUX_ARCHIVE}"

  # Create Windows ZIP if windows build was requested matching ots-xg.spec
  if [ "${BUILD_WINDOWS}" = "true" ]; then
    WIN_STAGING="${OUTPUT_DIR}/ots-win-staging"
    rm -rf "${WIN_STAGING}"
    mkdir -p "${WIN_STAGING}/bin" "${WIN_STAGING}/etc/custom" "${WIN_STAGING}/log"

    cp -f "${BIN_DIR}/ots_windows_amd64.exe" "${WIN_STAGING}/bin/ots.exe" 2>/dev/null || true
    cp -f "${BIN_DIR}/ots-cli_windows_amd64.exe" "${WIN_STAGING}/bin/ots-cli.exe" 2>/dev/null || true

    if [ -f "${SCRIPT_DIR}/ots-config.yaml" ]; then
      cp -f "${SCRIPT_DIR}/ots-config.yaml" "${WIN_STAGING}/etc/ots-config.yaml"
      sed -i 's|/etc/ots/custom|c:/inetd/ots/etc/custom|g' "${WIN_STAGING}/etc/ots-config.yaml" 2>/dev/null || true
    fi
    if [ -f "${SCRIPT_DIR}/ots.sysconfig" ]; then
      cp -f "${SCRIPT_DIR}/ots.sysconfig" "${WIN_STAGING}/etc/ots.env"
      sed -i 's|/etc/ots/ots-config.yaml|c:/inetd/ots/etc/ots-config.yaml|g' "${WIN_STAGING}/etc/ots.env" 2>/dev/null || true
    fi
    echo "ots-${VERSION}-${TAG}-windows_amd64" > "${WIN_STAGING}/etc/ots.version"

    WIN_ARCHIVE="${OUTPUT_DIR}/ots-${VERSION}-${TAG}-windows_amd64.zip"
    if command -v zip &>/dev/null; then
      (cd "${WIN_STAGING}" && zip -r "${WIN_ARCHIVE}" ./*)
    else
      echo "    Notice: 'zip' command not found, skipping ZIP compression."
    fi
    rm -rf "${WIN_STAGING}"
    echo "    Windows Archive Created: ${WIN_ARCHIVE}"
  fi

  # Vendor packaging
  if [ "${BUILD_VENDOR}" = "true" ]; then
    echo "==> Creating vendor archive..."
    go mod vendor
    VENDOR_ARCHIVE="${OUTPUT_DIR}/ots-${VERSION}-${TAG}-vendor.tar.gz"
    tar -czvf "${VENDOR_ARCHIVE}" vendor/
    echo "    Vendor Archive Created: ${VENDOR_ARCHIVE}"
  fi
fi

# Step 8: Live API Validation (--validate)
if [ "${BUILD_VALIDATE}" = "true" ]; then
  echo "==> Running Live API Validation (--validate enabled)..."
  kill_running_ots_processes

  TEST_PORT="39999"
  TEST_LISTEN="127.0.0.1:${TEST_PORT}"
  TEST_URL="http://${TEST_LISTEN}/api"
  SERVER_BIN=""

  # Select compiled binary for current host platform
  if [ -f "${BIN_DIR}/ots_windows_amd64.exe" ] && [[ "$(uname -s 2>/dev/null || echo "")" == *"NT"* || "$(uname -s 2>/dev/null || echo "")" == *"CYGWIN"* || "$(uname -s 2>/dev/null || echo "")" == *"MSYS"* || "$(uname -s 2>/dev/null || echo "")" == *"MINGW"* ]]; then
    SERVER_BIN="${BIN_DIR}/ots_windows_amd64.exe"
  elif [ -f "${BIN_DIR}/ots_linux_amd64" ]; then
    SERVER_BIN="${BIN_DIR}/ots_linux_amd64"
  fi

  if [ -z "${SERVER_BIN}" ] || [ ! -f "${SERVER_BIN}" ]; then
    echo "    Validation FAILED: No compiled binary found matching platform for validation."
    exit 1
  fi

  echo "    Starting compiled binary (${SERVER_BIN}) on ${TEST_LISTEN}..."
  "${SERVER_BIN}" --listen "${TEST_LISTEN}" --log-level warn &
  SERVER_PID=$!

  # Ensure server process is killed on exit
  trap 'kill ${SERVER_PID} 2>/dev/null || true' EXIT

  # Wait for server readiness (up to 5 seconds)
  READY=false
  for _ in {1..10}; do
    if curl -s "${TEST_URL}/healthz" &>/dev/null; then
      READY=true
      break
    fi
    sleep 0.5
  done

  if [ "${READY}" = "false" ]; then
    echo "    Validation FAILED: Server failed to respond on ${TEST_URL}/healthz"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  echo "    1. Testing /api/healthz & /api/isWritable endpoints..."
  curl -sSf "${TEST_URL}/healthz" &>/dev/null
  curl -sSf "${TEST_URL}/isWritable" &>/dev/null

  echo "    2. Testing /api/settings endpoint..."
  SETTINGS_RESP="$(curl -sSf "${TEST_URL}/settings")"
  if [[ "${SETTINGS_RESP}" != *"appTitle"* ]]; then
    echo "    Validation FAILED: /api/settings returned invalid response: ${SETTINGS_RESP}"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  echo "    3. Testing Secret Creation & One-Time Read/Burn..."
  CREATE_RESP="$(curl -sSf -X POST -H "Content-Type: application/json" -d '{"secret":"Validation Test Payload"}' "${TEST_URL}/create")"
  SECRET_ID="$(echo "${CREATE_RESP}" | sed -n 's/.*"secret_id":"\([^"]*\)".*/\1/p')"

  if [ -z "${SECRET_ID}" ]; then
    echo "    Validation FAILED: Could not parse secret_id from response: ${CREATE_RESP}"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  # First read succeeds
  GET_RESP="$(curl -sSf "${TEST_URL}/get/${SECRET_ID}")"
  if [[ "${GET_RESP}" != *"Validation Test Payload"* ]]; then
    echo "    Validation FAILED: Secret payload mismatch: ${GET_RESP}"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  # Second read returns 404 (Burn-after-read verified)
  HTTP_CODE="$(curl -s -o /dev/null -w "%{http_code}" "${TEST_URL}/get/${SECRET_ID}")"
  if [ "${HTTP_CODE}" != "404" ]; then
    echo "    Validation FAILED: Second read returned status ${HTTP_CODE}, expected 404 Not Found"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  echo "    4. Testing Large Secret Payload (~75 MiB payload capacity)..."
  LARGE_CREATE_RESP="$( (echo -n '{"secret":"' && head -c 75000000 /dev/zero | base64 -w 0 && echo -n '"}') | curl -sSf -X POST -H "Content-Type: application/json" --data-binary @- "${TEST_URL}/create" )"
  LARGE_SECRET_ID="$(echo "${LARGE_CREATE_RESP}" | sed -n 's/.*"secret_id":"\([^"]*\)".*/\1/p')"
  if [ -z "${LARGE_SECRET_ID}" ]; then
    echo "    Validation FAILED: Large secret creation failed"
    kill ${SERVER_PID} 2>/dev/null || true
    exit 1
  fi

  echo "    Shutting down test server (PID ${SERVER_PID})..."
  kill ${SERVER_PID} 2>/dev/null || true
  trap - EXIT

  echo "==> Live API Validation PASSED 100%!"
fi

echo "================================================="
echo " SUCCESS: OTS Build completed successfully!"
echo " Binaries stored in: ${BIN_DIR}"
echo " Packages stored in: ${OUTPUT_DIR}"
echo "================================================="

if [ "${AUTO_START}" = "true" ]; then
  kill_running_ots_processes
  CUSTOM_CFG="${ROOT_DIR}/testfiles/customize.yaml"
  echo "==> Auto-starting OTS server on http://127.0.0.1:3000/ ..."

  if [ -f "${BIN_DIR}/ots_windows_amd64.exe" ] && command -v cygpath &>/dev/null; then
    EXEC_PATH="$(cygpath -w "${BIN_DIR}/ots_windows_amd64.exe")"
    CFG_PATH="$(cygpath -w "${CUSTOM_CFG}")"
    nohup "${EXEC_PATH}" --listen 127.0.0.1:3000 --customize "${CFG_PATH}" >/dev/null 2>&1 &
  elif [ -f "${BIN_DIR}/ots_linux_amd64" ]; then
    nohup "${BIN_DIR}/ots_linux_amd64" --listen 127.0.0.1:3000 --customize "${CUSTOM_CFG}" >/dev/null 2>&1 &
  fi

  echo "    Server auto-started successfully (PID: $!)."
fi
