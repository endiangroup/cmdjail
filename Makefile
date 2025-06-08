###############################################################################
# Tooling configuration. 
#
# All executable names should be defined as variables so that they may be 
# overloaded.
###############################################################################

GO            ?= go
GOOS          ?= $(shell go env GOOS)
GOARCH        ?= $(shell go env GOARCH)

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

bin/cmdjail: bin *.go
	@mkdir -p bin
	@$(GO) build $(BUILD_VERBOSE_CONTROL) $(BUILD_CACHE_CONTROL) -o bin/cmdjail .

#-- Building

.PHONY: build
build: ## Build binary for a specific platform (e.g., GOOS=linux GOARCH=arm64 make build)
	@mkdir -p build
	@echo "Building for $(GOOS)/$(GOARCH)..."
	@GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 $(GO) build $(BUILD_VERBOSE_CONTROL) $(BUILD_CACHE_CONTROL) -o build/cmdjail-$(GOOS)-$(GOARCH) .

#-- Testing
.PHONY: test test-units test-features
test: test-units test-features ## test entire application

test-units:
	@$(GO) test $(TEST_VERBOSE_CONTROL) $(TEST_CACHE_CONTROL) ./...

test-features: bin/cmdjail
	@shellspec $(SHELLSPEC_VERBOSE_CONTROL)

.PHONY: clean
clean:
	rm -rf bin
	rm -rf build

