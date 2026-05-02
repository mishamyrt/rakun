VERSION = 0.1.0

APP_VERSION := $(shell \
	if ! git diff --quiet || \
	   ! git diff --cached --quiet || \
	   test -n "$$(git ls-files --others --exclude-standard)"; then \
			echo dev; \
	else \
			echo $(VERSION); \
	fi \
)
GC = go build -ldflags="-s -w -X rakun/cmd/main.version=$(APP_VERSION)"
ENTRYFILE = cmd/rakun.go

BUILD_DIR = build
BINARY_NAME = rakun

LINUX_ARM32 = $(BUILD_DIR)/linux/arm32/$(BINARY_NAME)
LINUX_ARM64 = $(BUILD_DIR)/linux/arm64/$(BINARY_NAME)
LINUX_AMD64 = $(BUILD_DIR)/linux/amd64/$(BINARY_NAME)
DARWIN_ARM64 = $(BUILD_DIR)/darwin/arm64/$(BINARY_NAME)
DARWIN_AMD64 = $(BUILD_DIR)/darwin/amd64/$(BINARY_NAME)


all: \
	$(LINUX_ARM32) \
	$(LINUX_ARM64) \
	$(LINUX_AMD64) \
	$(DARWIN_ARM64 ) \
	$(DARWIN_AMD64)

define build_binary
    mkdir -p "$(dir $(1))" && \
    env GOOS="$(2)" GOARCH="$(3)" \
    	$(GC) \
     		-o "$(1)" \
       		"$(ENTRYFILE)"
endef

.PHONY: $(LINUX_ARM32)
$(LINUX_ARM32): $(GOSRC)
	$(call build_binary,$(LINUX_ARM32),linux,arm)

.PHONY: $(LINUX_ARM64)
$(LINUX_ARM64): $(GOSRC)
	$(call build_binary,$(LINUX_ARM64),linux,arm64)

.PHONY: $(LINUX_AMD64)
$(LINUX_AMD64): $(GOSRC)
	$(call build_binary,$(LINUX_AMD64),linux,amd64)

.PHONY: $(DARWIN_ARM64)
$(DARWIN_ARM64): $(GOSRC)
	$(call build_binary,$(DARWIN_ARM64),darwin,arm64)

.PHONY: $(DARWIN_AMD64)
$(DARWIN_AMD64): $(GOSRC)
	$(call build_binary,$(DARWIN_AMD64),darwin,amd64)

.PHONY: test
test:
	go test -v ./...

.PHONY: lint
lint:
	golangci-lint run ./...
	revive -config ./revive.toml  ./...

.PHONY: check
check: lint test

.PHONY: publish
publish:
	git add Makefile
	git commit -m "chore: release ${VERSION} 🔥"
	git tag "v${VERSION}"
	git-cliff -o CHANGELOG.md
	git tag -d "v${VERSION}"
	git add CHANGELOG.md
	git commit --amend --no-edit
	git tag -a "v${VERSION}" -m "release v${VERSION}"
	git push
	git push --tags
