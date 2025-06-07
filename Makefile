###############################################################################
# Tooling configuration. 
#
# All executable names should be defined as variables so that they may be 
# overloaded.
###############################################################################

GO            ?= go

# Use NOCACHE=1 to disable go test cache e.g.
# 	$ NOCACHE=1 make test
ifdef NOCACHE
	TEST_CACHE_CONTROL ?= -count=1
endif

# Use VERBOSE=1 to enable test output verbosity e.g.
# 	$ VERBOSE=1 make test
ifdef VERBOSE
	TEST_VERBOSE_CONTROL      ?= -v
	LINT_VERBOSE_CONTROL      ?= -v
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
	@$(GO) build -o bin/cmdjail .

#-- Building

.PHONY: build
build: build/cmdjail-linux-arm64 ## build binaries across platform

build/cmdjail-linux-arm64: *.go
	@mkdir -p build
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GO) build -o build/cmdjail-linux-arm64 .

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

