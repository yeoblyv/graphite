MODULE  := github.com/yeoblyv/graphite
DIST    := dist
PROGRAMS := demo:./demo

.PHONY: build build-all \
	build-windows-amd64 build-windows-386 build-linux-amd64 build-linux-386 build-target \
	test vet fmt lint tidy clean

## build: compile every package for the host GOOS/GOARCH (no cross-compile).
build:
	go build ./...

## build-all: cross-compile every program in PROGRAMS for every supported target.
build-all: build-windows-amd64 build-windows-386 build-linux-amd64 build-linux-386

build-windows-amd64:
	@$(MAKE) --no-print-directory build-target GOOS=windows GOARCH=amd64 EXT=.exe

build-windows-386:
	@$(MAKE) --no-print-directory build-target GOOS=windows GOARCH=386 EXT=.exe

build-linux-amd64:
	@$(MAKE) --no-print-directory build-target GOOS=linux GOARCH=amd64 EXT=

build-linux-386:
	@$(MAKE) --no-print-directory build-target GOOS=linux GOARCH=386 EXT=

# build-target is the shared cross-compile step every build-<os>-<arch>
# target delegates to; GOOS/GOARCH/EXT are supplied by the caller above.
build-target:
	@for prog in $(PROGRAMS); do \
		name=$${prog%%:*}; \
		path=$${prog#*:}; \
		out=$(DIST)/$(GOOS)_$(GOARCH)/$$name$(EXT); \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $$out $$path || exit 1; \
	done

## test: run the test suite.
test:
	go test ./...

## vet: run go vet.
vet:
	go vet ./...

## fmt: fail if any file is not gofmt-formatted.
fmt:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

## lint: run golangci-lint if it is installed, warn otherwise.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping (see https://golangci-lint.run)"; \
	fi

## tidy: run go mod tidy.
tidy:
	go mod tidy

## clean: remove build output.
clean:
	rm -rf $(DIST)
