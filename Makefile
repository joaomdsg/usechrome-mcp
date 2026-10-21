# The pipeline's checks, runnable locally with the same pinned tools. CI calls
# these targets, so a green `make commit-stage acceptance` predicts a green run.

tool = go tool -modfile=devtools/$(1)/go.mod $(1)
BIN := dist/usechrome-mcp
COMMITS ?= origin/main..HEAD
# The pipeline sets the tag on a release run; .goreleaser.yaml builds with
# the same flags, so the release reproduces the tested binary.
VERSION ?= dev
IMAGE ?= usechrome-mcp:acceptance
PLATFORM := linux/$(shell go env GOARCH)

.PHONY: commit-stage build tidy vet test coverage lint vuln secrets workflows release-check release commit-messages acceptance image acceptance-image readme

commit-stage: build tidy vet test coverage lint vuln secrets workflows release-check

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/usechrome-mcp

tidy:
	go mod tidy -diff
	for d in devtools/*/; do (cd $$d && go mod tidy -diff) || exit 1; done

vet:
	go vet ./...
	go vet -tags acceptance ./acceptance/...

test:
	go test -race -count=1 -timeout 2m -coverprofile=cover.out ./...

# Reads the cover.out that `test` writes.
coverage:
	$(call tool,go-test-coverage) --config .testcoverage.yml

lint:
	$(call tool,golangci-lint) run ./...

vuln:
	$(call tool,govulncheck) ./...

secrets:
	$(call tool,gitleaks) git --redact --no-banner

workflows:
	$(call tool,actionlint)

release-check:
	$(call tool,goreleaser) check

# The pipeline's release job; it needs a version tag on HEAD and credentials.
release:
	$(call tool,goreleaser) release --clean

commit-messages:
	scripts/check-commit-messages.sh $(COMMITS)

acceptance:
	USECHROME_MCP_BIN=$(CURDIR)/$(BIN) go test -tags acceptance -count=1 -timeout 5m ./acceptance/...

# Builds the release image around $(BIN), in the context layout GoReleaser uses.
image:
	rm -rf dist/image
	mkdir -p dist/image/$(PLATFORM)
	cp $(BIN) dist/image/$(PLATFORM)/
	docker build --platform $(PLATFORM) -t $(IMAGE) -f Dockerfile dist/image

acceptance-image: image
	USECHROME_MCP_BIN=$(CURDIR)/scripts/run-image.sh USECHROME_MCP_IMAGE=$(IMAGE) go test -tags acceptance -count=1 -timeout 5m ./acceptance/...

readme:
	go test ./cmd/usechrome-mcp -run TestReadme -count=1 -update
