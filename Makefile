###############################################################################
# Tooling configuration. 
#
# All executable names should be defined as variables so that they may be 
# overloaded.
###############################################################################

GO            ?= go
GOLANGCI_LINT ?= golangci-lint
GOOS          ?= $(shell go env GOOS)
GOARCH        ?= $(shell go env GOARCH)
FUZZ_TIME     ?= 30s

# Versioning
GIT_VERSION   ?= $(shell git describe --tags --always --dirty)
GIT_COMMIT    ?= $(shell git rev-parse HEAD)
BUILD_DATE    ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS       := -ldflags="-X 'main.version=$(GIT_VERSION)' -X 'main.commit=$(GIT_COMMIT)' -X 'main.date=$(BUILD_DATE)'"

# Use NOCACHE=1 to disable go test cache e.g.
# 	$ NOCACHE=1 make test
ifdef NOCACHE
	TEST_CACHE_CONTROL  ?= -count=1
	BUILD_CACHE_CONTROL ?= -a
endif

# Use VERBOSE=1 to enable test output verbosity e.g.
# 	$ VERBOSE=1 make test
ifdef VERBOSE
	TEST_VERBOSE_CONTROL      ?= -v
	LINT_VERBOSE_CONTROL      ?= -v
	BUILD_VERBOSE_CONTROL     ?= -v
	SHELLSPEC_VERBOSE_CONTROL ?= --format documentation
endif

###############################################################################
# Development targets
###############################################################################

help:
	@grep -E '^[1-9a-zA-Z_-]+:.*?## .*$$|(^#--)' $(MAKEFILE_LIST) \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[32m %-43s\033[0m %s\n", $$1, $$2}' \
	| sed -e 's/\[32m #-- /[33m/'

bin/cmdjail: *.go
	@mkdir -p bin
	@$(GO) build $(LDFLAGS) $(BUILD_VERBOSE_CONTROL) $(BUILD_CACHE_CONTROL) -o bin/cmdjail .

#-- Building

.PHONY: build
build: ## Build binary for a specific platform (e.g., GOOS=linux GOARCH=arm64 make build)
	@mkdir -p build
	@echo "Building for $(GOOS)/$(GOARCH)..."
	@GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 $(GO) build $(LDFLAGS) $(BUILD_VERBOSE_CONTROL) $(BUILD_CACHE_CONTROL) -o build/cmdjail-$(GOOS)-$(GOARCH) .

#-- Linting
.PHONY: lint
lint: ## Run golangci-lint
	@$(GOLANGCI_LINT) run $(LINT_VERBOSE_CONTROL) ./...

#-- Testing
.PHONY: test test-units test-features
test: test-units test-features ## test entire application

test-units:
	@$(GO) test $(TEST_VERBOSE_CONTROL) $(TEST_CACHE_CONTROL) ./...

test-features: bin/cmdjail
	@shellspec $(SHELLSPEC_VERBOSE_CONTROL)

.PHONY: test-fuzz test-fuzz-parser test-fuzz-evaluator
test-fuzz: test-fuzz-parser test-fuzz-evaluator ## Run all fuzz tests for a short duration (default: 10s each)

test-fuzz-parser: ## Run fuzz tests for the jailfile parser
	@echo "Fuzzing jailfile parser for $(FUZZ_TIME)..."
	@$(GO) test $(TEST_VERBOSE_CONTROL) $(TEST_CACHE_CONTROL) -fuzz=FuzzParseJailFile -fuzztime=$(FUZZ_TIME) .

test-fuzz-evaluator: ## Run fuzz tests for the command evaluator
	@echo "Fuzzing command evaluator for $(FUZZ_TIME)..."
	@$(GO) test $(TEST_VERBOSE_CONTROL) $(TEST_CACHE_CONTROL) -fuzz=FuzzEvaluateCmd -fuzztime=$(FUZZ_TIME) .

.PHONY: clean
clean:
	rm -rf bin
	rm -rf build
