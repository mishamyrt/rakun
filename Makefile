VERSION = 0.1.1
APP_VERSION := $(shell \
	if ! git diff --quiet || \
	   ! git diff --cached --quiet || \
	   test -n "$$(git ls-files --others --exclude-standard)"; then \
			echo dev; \
	else \
			echo $(VERSION); \
	fi \
)
GC = go build -ldflags="-s -w -X rakun/cmd.version=$(APP_VERSION)"

BUILD_OS = $(shell go env GOOS)
BUILD_ARCH = $(shell go env GOARCH)
BUILD_OUTPUT = build/rakun

all: build

.PHONY: build
build:
	env GOOS="$(BUILD_OS)" GOARCH="$(BUILD_ARCH)" \
		$(GC) -o "$(BUILD_OUTPUT)" "rakun.go"

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
