OTS_VERSION ?= dev
PRODUCT_VERSION := $(OTS_VERSION)

BIN_DIR := testfiles/bin
IS_WINDOWS := $(findstring Windows,$(OS))
EXEC_EXT := $(if $(IS_WINDOWS),.exe,)

PNPM := $(if $(IS_WINDOWS),cmd /c pnpm,pnpm)

OTS_BIN := $(BIN_DIR)/ots$(EXEC_EXT)
CLI_BIN := $(BIN_DIR)/ots-cli$(EXEC_EXT)
TRANSLATE_BIN := $(BIN_DIR)/translate$(EXEC_EXT)

default: build-local

build-local: export CGO_ENABLED=0
build-local: export SOURCE_DATE_EPOCH=1
build-local: frontend generate-apidocs
	go build \
		-buildvcs=false \
		-ldflags "-s -w -buildid= -X main.version=$(PRODUCT_VERSION)" \
		-mod=readonly \
		-trimpath \
		-o $(OTS_BIN)
	cd cmd/ots-cli && go build \
		-buildvcs=false \
		-ldflags "-s -w -buildid= -X main.version=$(PRODUCT_VERSION)" \
		-mod=readonly \
		-trimpath \
		-o "../../$(CLI_BIN)"
	cd ci/translate && go build \
		-buildvcs=false \
		-ldflags "-s -w -buildid= -X main.version=$(PRODUCT_VERSION)" \
		-mod=readonly \
		-trimpath \
		-o "../../$(TRANSLATE_BIN)"

generate-apidocs: node_modules
	$(PNPM) redocly \
		--disableGoogleFont true \
		-o frontend/api.html \
		build-docs docs/openapi.yaml

frontend_prod: export NODE_ENV=production
frontend_prod: frontend

frontend: node_modules
	$(PNPM) node ci/build.mjs

frontend_lint: node_modules
	$(PNPM) eslint --fix src

node_modules:
	$(PNPM) install --production=false --frozen-lockfile

publish: export NODE_ENV=production
publish: frontend_prod generate-apidocs
	bash ./ci/packager.sh

translate:
	$(TRANSLATE_BIN)

# -- Testing (elevated timeouts for Windows WSL Docker & VM latency) --

IS_WINDOWS := $(findstring Windows,$(OS))

test:
	go test -v ./...
	cd cmd/ots-cli && go test -v .
ifdef IS_WINDOWS
	cmd /c pnpm test:ui
else
	pnpm test:ui
endif

test-e2e:
	go test -v -tags=e2e -timeout 20m ./...
	cd cmd/ots-cli && go test -v -tags=e2e -timeout 20m .
ifdef IS_WINDOWS
	cmd /c pnpm test:ui:e2e
else
	pnpm test:ui:e2e
endif

test-ui:
ifdef IS_WINDOWS
	cmd /c pnpm test:ui
else
	pnpm test:ui
endif

test-ui-e2e:
ifdef IS_WINDOWS
	cmd /c pnpm test:ui:e2e
else
	pnpm test:ui:e2e
endif

# -- Vulnerability scanning --

trivy:
	trivy fs . \
		--dependency-tree \
		--exit-code 1 \
		--format table \
		--ignore-unfixed \
		--quiet \
		--scanners config,license,secret,vuln \
		--severity HIGH,CRITICAL \
		--skip-dirs docs,node_modules

.PHONY: node_modules translate
