export GO15VENDOREXPERIMENT:=1
export CGO_ENABLED:=0

GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor|internal/templates.go)')
GOPACKAGES:=$(shell go list ./... | grep -v '/vendor/')

all: bin/bootkube

fmt:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -d {} +

vet:
	@go vet $(GOPACKAGES)

bin/bootkube: $(GOFILES) pkg/assets/internal/templates.go
	mkdir -p bin
	go build -o bin/bootkube github.com/coreos/bootkube/cmd/bootkube

pkg/assets/internal/templates.go: $(GOFILES)
	mkdir -p $(dir $@)
	go generate pkg/assets/assets.go

clean:
	rm -f bin/bootkube
	rm -rf pkg/assets/internal

.PHONY: all clean fmt vet

