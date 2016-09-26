export GO15VENDOREXPERIMENT:=1
export CGO_ENABLED:=0
export GOARCH:=amd64

LOCAL_OS:=$(shell uname | tr A-Z a-z)
GOFILES:=$(shell find . -name '*.go' | grep -v -E '(./vendor)')
GOPATH_BIN:=$(shell echo ${GOPATH} | awk 'BEGIN { FS = ":" }; { print $1 }')/bin
LDFLAGS=-X github.com/kubernetes-incubator/bootkube/pkg/version.Version=$(shell $(CURDIR)/build/git-version.sh)

all: \
	_output/bin/linux/bootkube \
	_output/bin/darwin/bootkube \
	_output/bin/linux/checkpoint \
	_output/bin/linux/node-agent \
	_output/bin/linux/update-controller

release: clean check \
	_output/release/bootkube.tar.gz

check:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/')

install: _output/bin/$(LOCAL_OS)/bootkube
	cp $< $(GOPATH_BIN)

_output/bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	GOOS=$(word 1, $(subst /, ,$*)) go build -ldflags "$(LDFLAGS)" -o $@ github.com/kubernetes-incubator/bootkube/cmd/$(notdir $@)

_output/release/bootkube.tar.gz: _output/bin/linux/bootkube _output/bin/darwin/bootkube _output/bin/linux/checkpoint _output/bin/linux/node-agent _output/bin/linux/update-controller
	mkdir -p $(dir $@)
	tar czf $@ -C _output bin/linux/bootkube bin/darwin/bootkube bin/linux/checkpoint

#TODO(aaron): Prompt because this is destructive
conformance-%: clean all
	@cd hack/$*-node && vagrant destroy -f
	@cd hack/$*-node && rm -rf cluster
	@cd hack/$*-node && ./bootkube-up
	@sleep 30 # Give addons a little time to start
	@cd hack/$*-node && ./conformance-test.sh

# This will naively try and create a vendor dir from a k8s release
# USE: make vendor VENDOR_VERSION=vX.Y.Z
#TODO(aaron): Prompt because this is destructive
VENDOR_VERSION = v1.3.6+coreos.0
vendor:
	@echo "Creating k8s vendor for: $(VENDOR_VERSION)"
	@rm -rf vendor
	@mkdir -p $@/k8s.io/kubernetes
	@git clone https://github.com/coreos/kubernetes $@/k8s.io/kubernetes > /dev/null 2>&1
	@cd $@/k8s.io/kubernetes && git checkout $(VENDOR_VERSION) > /dev/null 2>&1
	@cd $@/k8s.io/kubernetes && rm -rf docs examples hack cluster Godeps
	@cd $@/k8s.io/kubernetes/vendor && mv k8s.io/heapster $(abspath $@/k8s.io) && rmdir k8s.io
	@mv $@/k8s.io/kubernetes/vendor/* $(abspath $@)
	@rm -rf $@/k8s.io/kubernetes/vendor $@/k8s.io/kubernetes/.git

clean:
	rm -rf _output

.PHONY: all check clean install release vendor
