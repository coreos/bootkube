export GO15VENDOREXPERIMENT:=1
export CGO_ENABLED:=0
export GOARCH:=amd64

LOCAL_OS:=$(shell uname | tr A-Z a-z)
GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor|internal/templates.go)')
TEMPLATES:=$(shell find pkg/asset/templates -type f)
GOPATH_BIN:=$(shell echo ${GOPATH} | awk 'BEGIN { FS = ":" }; { print $1 }')/bin

.PHONY: all
all: \
	_output/bin/linux/bootkube \
	_output/bin/darwin/bootkube \
	_output/bin/linux/checkpoint

.PHONY: release
release: clean check \
	_output/release/bootkube.tar.gz

.PHONY: templates
templates: $(GOFILES) pkg/asset/internal/templates.go

.PHONY: check
check: pkg/asset/internal/templates.go
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/')

.PHONY: install
install: _output/bin/$(LOCAL_OS)/bootkube
	cp $< $(GOPATH_BIN)

_output/bin/%/bootkube: LDFLAGS=-X github.com/coreos/bootkube/pkg/version.Version=$(shell $(CURDIR)/build/git-version.sh)
_output/bin/%/bootkube: $(GOFILES) pkg/asset/internal/templates.go
	mkdir -p $(dir $@)
	GOOS=$* go build -ldflags "$(LDFLAGS)" -o _output/bin/$*/bootkube github.com/coreos/bootkube/cmd/bootkube

_output/bin/%/checkpoint: cmd/checkpoint/main.go
	mkdir -p $(dir $@)
	GOOS=$* go build -o _output/bin/$*/checkpoint github.com/coreos/bootkube/cmd/checkpoint

_output/release/bootkube.tar.gz: _output/bin/linux/bootkube _output/bin/darwin/bootkube _output/bin/linux/checkpoint
	mkdir -p $(dir $@)
	tar czf $@ -C _output bin/linux/bootkube bin/darwin/bootkube bin/linux/checkpoint

pkg/asset/internal/templates.go: $(GOFILES) $(TEMPLATES)
	mkdir -p $(dir $@)
	go generate pkg/asset/templates_gen.go

#TODO(aaron): Prompt because this is destructive
conformance-%: clean all
	@cd hack/$*-node && vagrant destroy -f
	@cd hack/$*-node && rm -rf cluster
	@cd hack/$*-node && ./bootkube-up
	@sleep 30 # Give addons a little time to start
	@cd hack/$*-node && ./conformance-test.sh

# This will naively try and create a vendor dir from a k8s release
# USE: make vendor VENDOR_VERSION=vX.Y.Z
VENDOR_VERSION = coreos-hyperkube-v1.4.0-alpha.3

.PHONY: vendor
vendor: vendor-$(VENDOR_VERSION)

#TODO(aaron): the k8s.io/client-go upstream package is a symlink with relative path.
# Just making note here because we change the symlink path -- but this is all likely temporary
vendor-$(VENDOR_VERSION):
	@echo "Creating k8s vendor dir: $@"
	@mkdir -p $@/k8s.io/kubernetes
	@git clone --branch=$(VENDOR_VERSION) --depth=1 https://github.com/coreos/kubernetes $@/k8s.io/kubernetes > /dev/null 2>&1
	@cd $@/k8s.io/kubernetes && git checkout $(VENDOR_VERSION) > /dev/null 2>&1
	@cd $@/k8s.io/kubernetes && rm -rf docs examples hack cluster Godeps
	@cd $@/k8s.io/kubernetes/vendor && mv k8s.io/* $(abspath $@/k8s.io) && rmdir k8s.io
	@mv $@/k8s.io/kubernetes/vendor/* $(abspath $@)
	@cd $@/k8s.io/ && ln -sf kubernetes/staging/src/k8s.io/client-go client-go
	@rm -rf $@/k8s.io/kubernetes/vendor $@/k8s.io/kubernetes/.git

.PHONY: clean
clean:
	rm -rf _output
	rm -rf pkg/asset/internal
