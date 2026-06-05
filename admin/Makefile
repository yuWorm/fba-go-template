GO ?= go
APP ?= fba-admin-api
BIN_DIR ?= bin
GOCACHE_DIR ?= .cache/go-build

# L=1 时使用 go.local.mod
L ?=
MODFLAGS := $(if $(L),-modfile=go.local.mod,)

GOENV := GOWORK=off GOCACHE=$(CURDIR)/$(GOCACHE_DIR)

.PHONY: tidy
tidy:
	$(GOENV) $(GO) mod tidy $(MODFLAGS)

.PHONY: test
test:
	$(GOENV) $(GO) test $(MODFLAGS) ./...

.PHONY: run
run:
	$(GOENV) $(GO) run $(MODFLAGS) ./cmd/api

.PHONY: dev
dev: run

.PHONY: build
build:
	mkdir -p $(BIN_DIR)
	$(GOENV) $(GO) build $(MODFLAGS) -o $(BIN_DIR)/$(APP) ./cmd/api

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) .cache
