# The pipeline's checks, runnable locally with the same pinned tools. CI calls
# these targets, so a green `make commit-stage acceptance` predicts a green run.

tool = go tool -modfile=devtools/$(1)/go.mod $(1)
BIN := dist/usechrome-mcp
COMMITS ?= origin/main..HEAD

.PHONY: commit-stage build tidy vet test coverage lint vuln secrets workflows commit-messages acceptance

commit-stage: build tidy vet test coverage lint vuln secrets workflows

build:
	CGO_ENABLED=0 go build -trimpath -o $(BIN) ./cmd/usechrome-mcp

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

commit-messages:
	scripts/check-commit-messages.sh $(COMMITS)

acceptance:
	USECHROME_MCP_BIN=$(CURDIR)/$(BIN) go test -tags acceptance -count=1 -timeout 5m ./acceptance/...
