.DEFAULT_GOAL := all

GOPKG := github.com/jonathonwebb/tinycli

TMPDIR := tmp

## all: run development tasks (default target)
.PHONY: all
all: deps fmt vet test

.PHONY: check
check: deps-check fmt-check vet cover-check

## deps: clean deps
.PHONY: deps
deps:
	go mod tidy -v

.PHONY: deps-check
deps-check:
	go mod tidy -diff
	go mod verify

## fmt: go fmt
.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: fmt-check
fmt-check:
	test -z "$(shell gofmt -l .)"

## vet: go vet
.PHONY: vet
vet:
	go vet ./...

## test: go test
.PHONY: test
test:
	go test ./...

.PHONY: test-check
test-check:
	go test -count=1 -v ./...

## cover: go test coverage
.PHONY: cover
cover: $(TMPDIR)
	go test -v -coverprofile $(TMPDIR)/cover.out $(GOPKG)
	go tool cover -html=$(TMPDIR)/cover.out

.PHONY: cover-check
cover-check: $(TMPDIR)
	go test -count=1 -v -coverprofile $(TMPDIR)/cover.out $(GOPKG)

## clean: clean output
.PHONY: clean
clean:
	rm -rf $(TMPDIR)

$(TMPDIR):
	mkdir -p $@

## help: display this help
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
