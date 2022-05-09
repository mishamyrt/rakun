.PHONY: all

GC = go build -ldflags="-s -w"
ENTRYFILE = cmd/git-sync/git-sync.go

BUILD_DIR = build
BINARY_NAME = git-sync

PATH_LINUX_ARM32 = $(BUILD_DIR)/linux/arm32/$(BINARY_NAME)
PATH_LINUX_ARM64 = $(BUILD_DIR)/linux/arm64/$(BINARY_NAME)
PATH_LINUX_AMD64 = $(BUILD_DIR)/linux/amd64/$(BINARY_NAME)
PATH_DARWIN_ARM64 = $(BUILD_DIR)/darwin/arm64/$(BINARY_NAME)
PATH_DARWIN_AMD64 = $(BUILD_DIR)/darwin/amd64/$(BINARY_NAME)

all: \
	$(PATH_LINUX_ARM32) \
	$(PATH_LINUX_ARM64) \
	$(PATH_LINUX_AMD64) \
	$(PATH_DARWIN_ARM64 ) \
	$(PATH_DARWIN_AMD64)

define build_binary
    env GOOS=$(2) GOARCH=$(3) $(GC) -o $(1) $(ENTRYFILE)
endef

GOSRC := $(wildcard cmd/*/**.go) $(wildcard internal/*/**.go)

$(PATH_LINUX_ARM32): $(GOSRC)
	$(call build_binary,$(PATH_LINUX_ARM32),linux,arm)

$(PATH_LINUX_ARM64): $(GOSRC)
	$(call build_binary,$(PATH_LINUX_ARM64),linux,arm64)

$(PATH_LINUX_AMD64): $(GOSRC)
	$(call build_binary,$(PATH_LINUX_AMD64),linux,amd64)

$(PATH_DARWIN_ARM64): $(GOSRC)
	$(call build_binary,$(PATH_DARWIN_ARM64),darwin,arm64)

$(PATH_DARWIN_AMD64): $(GOSRC)
	$(call build_binary,$(PATH_DARWIN_AMD64),darwin,amd64)

