#!/bin/bash

# ---------------------------
#  ots_builder.sh
#  v1.1.9xg  2026/06/18  XDG
# ---------------------------
# Syntax: ots_builder.sh <version>-<tag> [--no-package|--build-vendor|--windows]
# Example: ots_builder.sh 1.21.6-0289715 --build-vendor (needed for Veracode scan)

BUILD_ID="ots"
BUILD_IMAGE="node:24-alpine"
ALPINE_BRANCH="3.24"
ALPINE_RELEASE="${ALPINE_BRANCH}.1"
SOURCE_DIR="/usr/src/redhat/SOURCES"
OTS_BASE="${BUILD_ID}"
OTS_REPO="https://github.com/Luzifer/${BUILD_ID}"
OTS_LISTENER="127.0.0.1:3000"
OTS_VERSION_FULL="$1"
OTS_VERSION="${OTS_VERSION_FULL%-*}"
FONTAWESOME_REPO="https://github.com/FortAwesome/Font-Awesome/archive"
FONTAWESOME_VER="7.2.0"
DISTRIB_TARGET="/opt/done"
OS_PLATFORM="linux_amd64"
BIN_NAME="ots_${OS_PLATFORM}"
CLI_NAME="ots-cli_${OS_PLATFORM}"

BUILD_WINDOWS=false
NO_PACKAGE=false
BUILD_VENDOR=false

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
  esac
done

buildCode() {
  go get -u ./...
  go mod verify
  go mod tidy
  GOOS=$1 GOARCH=amd64 CGO_ENABLED=0 go build -v -tags release -buildmode=pie -mod=readonly -trimpath -ldflags "-s -w -X main.version=${OTS_VERSION}-${OTS_TAG}" -o "${BIN_PATH}/$2"
}

[ -z "${OTS_VERSION}" ] && exit 1

if [ "$NO_PACKAGE" = "false" ]; then
  cd "${SOURCE_DIR}" || exit 1
fi
sudo rm -rf "${OTS_BASE}"
git clone "${OTS_REPO}.git"

cd "${OTS_BASE}" || exit 1
CURDIR="$(pwd)"
BIN_PATH="${CURDIR}/bin"
OTS_TAG="${OTS_VERSION_FULL#*-}"
BUILD_LABEL="[$(date '+%Y%m%d-%H%M%S')|${BUILD_ID}-${OTS_VERSION}-${OTS_TAG}]"
BIN_ARCHNAME="ots-${OTS_VERSION}-${OTS_TAG}-${OS_PLATFORM}"

sed -i -n '/^  ca:/q;p' i18n.yaml
printf "\nBuilding %s release: %s\n" "${BUILD_ID}" "${OTS_VERSION_FULL}"
mkdir -p "${BIN_PATH}"
for p in "${SOURCE_DIR}/${BUILD_ID}"_*.patch; do
  if [ -f "$p" ]; then
    echo "Applying ${p}"
    patch -p1 < "${p}" || {
      echo "*** Error: patch failed"
      exit 2
    }
  fi
done

(cd ci/translate && go build)
./ci/translate/translate
rm -f ci/translate/translate

printf "\n%s Generating NodeJS Frontend\n" "${BUILD_LABEL}"
docker run --rm -i \
  -e ALPINE_BRANCH="${ALPINE_BRANCH}" -e ALPINE_RELEASE="${ALPINE_RELEASE}" \
  -v "${CURDIR}:${CURDIR}" -w "${CURDIR}" "${BUILD_IMAGE}" sh -exc "\
  sed -i \"s|/v\d\..*/|/v${ALPINE_BRANCH}/|g\" /etc/apk/repositories;\
  sed -i \"s|\d\..*|${ALPINE_RELEASE}|1\" /etc/alpine-release;\
  sed -i \"/^Welcome/s|\d\..*|${ALPINE_BRANCH}|1\" /etc/issue;\
  sed -i \"/^VERSION_ID/s|\d\.\d\{1,\}.*|${ALPINE_RELEASE}|1;/^PRETTY_NAME/s|v\d\.\d\{1,\}|v${ALPINE_BRANCH}|1\" /etc/os-release;\
  apk add --upgrade alpine-keys --allow-untrusted && apk update && apk upgrade --available;\
  npm install -g npm@latest pnpm >/dev/null 2>&1;\
  node --version;npm --version;pnpm --version;\
  apk add make;\
  make frontend_prod && chown -R $(id -u) ."

printf "\n%s Downloading Fonts (Font-Awesome)\n" "${BUILD_LABEL}"
rm -rf frontend/{css,js,webfonts}
curl -sSfL "${FONTAWESOME_REPO}/${FONTAWESOME_VER}.tar.gz" | \
  tar -vC frontend -xz --strip-components=1 --wildcards --exclude='*/js-packages' '*/css' '*/webfonts'

printf "\n%s Compiling Golang backend\n" "${BUILD_LABEL}"
buildCode linux "${BIN_NAME}"
(cd cmd/ots-cli && buildCode linux "${CLI_NAME}")
strip -s "${BIN_PATH}/${BIN_NAME}" "${BIN_PATH}/${CLI_NAME}"

if [ "$BUILD_WINDOWS" = "true" ]; then
  printf "\n%s Compiling Windows backend\n" "${BUILD_LABEL}"
  OS_PLATFORM_WIN="windows_amd64"
  BIN_NAME_WIN="ots_${OS_PLATFORM_WIN}.exe"
  CLI_NAME_WIN="ots-cli_${OS_PLATFORM_WIN}.exe"
  BIN_ARCHNAME_WIN="ots-${OTS_VERSION}-${OTS_TAG}-${OS_PLATFORM_WIN}"

  buildCode windows "${BIN_NAME_WIN}"
  (cd cmd/ots-cli && buildCode windows "${CLI_NAME_WIN}")
fi

"${BIN_PATH}/${BIN_NAME}" --version
"${BIN_PATH}/${CLI_NAME}" help | head -1
"${BIN_PATH}/${BIN_NAME}" ots_linux_amd64 --listen "${OTS_LISTENER}" &
curl -s "http://${OTS_LISTENER}" | pandoc -f html -t plain
killall "${BIN_NAME}" >/dev/null 2>&1 || true

if [ "$NO_PACKAGE" = "false" ]; then
  printf "\n%s Creating minimal archive (binaries only)\n" "${BUILD_LABEL}"
  tar Jcvf "${DISTRIB_TARGET}/${BIN_ARCHNAME}-bin.tar.xz" -C "${BIN_PATH}" "${BIN_NAME}" "${CLI_NAME}"

  if [ "$BUILD_WINDOWS" = "true" ]; then
    printf "\n%s Creating Windows ZIP archive\n" "${BUILD_LABEL}"
    DISTRIB_WIN="${CURDIR}/distrib_win"
    rm -rf "${DISTRIB_WIN}"
    mkdir -p "${DISTRIB_WIN}"/bin "${DISTRIB_WIN}"/etc/custom "${DISTRIB_WIN}"/log

    cp -af "${BIN_PATH}/${BIN_NAME_WIN}" "${DISTRIB_WIN}/bin/ots.exe"
    cp -af "${BIN_PATH}/${CLI_NAME_WIN}" "${DISTRIB_WIN}/bin/ots-cli.exe"

    cp -af "${SOURCE_DIR}/ots-config.yaml" "${DISTRIB_WIN}/etc/ots-config.yaml"
    cp -af "${SOURCE_DIR}/ots.sysconfig" "${DISTRIB_WIN}/etc/ots.env"
    sed -i 's|/etc/ots/ots-config.yaml|c:/inetd/ots/etc/ots-config.yaml|g' "${DISTRIB_WIN}/etc/ots.env"
    sed -i 's|/etc/ots/custom|c:/inetd/ots/etc/custom|g' "${DISTRIB_WIN}/etc/ots-config.yaml"
    echo "ots-${OTS_VERSION}-${OTS_TAG}-windows_amd64" > "${DISTRIB_WIN}/etc/ots.version"

    (cd "${DISTRIB_WIN}" && zip -r "${DISTRIB_TARGET}/${BIN_ARCHNAME_WIN}.zip" ./*)
    rm -rf "${DISTRIB_WIN}"
  fi

  if [ "$BUILD_VENDOR" = "true" ]; then
    go mod vendor
    cd .. || exit 1
    find "${OTS_BASE}" -type f -exec file {} \; | grep "ELF" | cut -d ":" -f1 | xargs rm -fv
    tar zcvf "${DISTRIB_TARGET}/${BIN_ARCHNAME}-vendor.tar.gz" "${OTS_BASE}"
  fi
fi
