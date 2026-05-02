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
GC = go build -ldflags="-s -w -X main.version=$(APP_VERSION)"

all: build

.PHONY: build
build:
	$(GC) -o "build/rakun" "cmd/rakun.go"

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
