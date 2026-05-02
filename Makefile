GC = go build -ldflags="-s -w"
ENTRYFILE = cmd/rakun.go

BUILD_DIR = build
BINARY_NAME = rakun

LINUX_ARM32 = $(BUILD_DIR)/linux/arm32/$(BINARY_NAME)
LINUX_ARM64 = $(BUILD_DIR)/linux/arm64/$(BINARY_NAME)
LINUX_AMD64 = $(BUILD_DIR)/linux/amd64/$(BINARY_NAME)
DARWIN_ARM64 = $(BUILD_DIR)/darwin/arm64/$(BINARY_NAME)
DARWIN_AMD64 = $(BUILD_DIR)/darwin/amd64/$(BINARY_NAME)

.PHONY: \
	clear \
	run \
	$(LINUX_ARM32) \
	$(LINUX_ARM64) \
	$(LINUX_AMD64) \
	$(DARWIN_ARM64) \
	$(DARWIN_AMD64)

all: \
	$(LINUX_ARM32) \
	$(LINUX_ARM64) \
	$(LINUX_AMD64) \
	$(DARWIN_ARM64 ) \
	$(DARWIN_AMD64)

define build_binary
    env GOOS="$(2)" GOARCH="$(3)" $(GC) -o "$(1)" "$(ENTRYFILE)"
endef

$(LINUX_ARM32): $(GOSRC)
	$(call build_binary,$(LINUX_ARM32),linux,arm)

$(LINUX_ARM64): $(GOSRC)
	$(call build_binary,$(LINUX_ARM64),linux,arm64)

$(LINUX_AMD64): $(GOSRC)
	$(call build_binary,$(LINUX_AMD64),linux,amd64)

$(DARWIN_ARM64): $(GOSRC)
	$(call build_binary,$(DARWIN_ARM64),darwin,arm64)

$(DARWIN_AMD64): $(GOSRC)
	$(call build_binary,$(DARWIN_AMD64),darwin,amd64)

.PHONY: lint
lint:
	golangci-lint run ./...
	revive -config ./revive.toml  ./...

.PHONY: test
test:
	go test ./...

.PHONY: check
check: lint test
