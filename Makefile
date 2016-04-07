export GO15VENDOREXPERIMENT:=1
export CGO_ENABLED:=0

GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor|internal/templates.go)')
GOPACKAGES:=$(shell go list ./... | grep -v '/vendor/')
GOPATH_BIN:=$(shell echo ${GOPATH} | awk 'BEGIN { FS = ":" }; { print $1 }')/bin

all: fmt vet bin/bootkube

fmt:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +

vet:
	@go vet $(GOPACKAGES)

# This will naively try and create a vendor dir from a k8s release
# USE: make vendor VENDOR_VERSION=vX.Y.Z
VENDOR_VERSION = v1.2.1
vendor: vendor-$(VENDOR_VERSION)

bin/bootkube: $(GOFILES) pkg/asset/internal/templates.go
	mkdir -p bin
	go build -o bin/bootkube github.com/coreos/bootkube/cmd/bootkube

install: all
	cp bin/bootkube $(GOPATH_BIN)

pkg/asset/internal/templates.go: $(GOFILES)
	mkdir -p $(dir $@)
	go generate pkg/asset/templates_gen.go

vendor-$(VENDOR_VERSION):
	@echo "Creating k8s vendor dir: $@"
	@mkdir -p $@/k8s.io/kubernetes
	@git clone --branch=$(VENDOR_VERSION) --depth=1 https://github.com/kubernetes/kubernetes $@/k8s.io/kubernetes > /dev/null 2>&1
	@cd $@/k8s.io/kubernetes && git checkout $(VENDOR_VERSION) > /dev/null 2>&1
	@cd $@/k8s.io/kubernetes && rm -rf docs examples hack cluster
	@cd $@/k8s.io/kubernetes/Godeps/_workspace/src && mv k8s.io/heapster $(abspath $@/k8s.io) && rmdir k8s.io
	@mv $@/k8s.io/kubernetes/Godeps/_workspace/src/* $(abspath $@)
	@rm -rf $@/k8s.io/kubernetes/Godeps $@/k8s.io/kubernetes/.git

clean:
	rm -f bin/bootkube
	rm -rf pkg/asset/internal

.PHONY: all clean fmt vet install

