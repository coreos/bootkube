export CGO_ENABLED:=0
export GOARCH:=amd64
export PATH:=$(PATH):$(PWD)

IMAGE_REPO_BOOTKUBE?=quay.io/coreos/bootkube
IMAGE_REPO_PODCHECKPOINTER?=quay.io/coreos/pod-checkpointer
VERSION?=

LOCAL_OS:=$(shell uname | tr A-Z a-z)
GOFILES:=$(shell find . -name '*.go' ! -path './vendor/*')
VENDOR_GOFILES ?= $(shell find vendor -name '*.go')
LDFLAGS=-X github.com/kubernetes-sigs/bootkube/pkg/version.Version=$(shell $(CURDIR)/build/git-version.sh)
TERRAFORM:=$(shell command -v terraform 2> /dev/null)

all: \
	_output/bin/$(LOCAL_OS)/bootkube \
	_output/bin/linux/bootkube \
	_output/bin/linux/checkpoint

cross: \
	_output/bin/linux/bootkube \
	_output/bin/linux/amd64/bootkube \
	_output/bin/linux/arm/bootkube \
	_output/bin/linux/arm64/bootkube \
	_output/bin/linux/ppc64le/bootkube \
	_output/bin/linux/s390x/bootkube \
	_output/bin/darwin/bootkube \
	_output/bin/darwin/amd64/bootkube \
	_output/bin/linux/checkpoint \
	_output/bin/linux/amd64/checkpoint \
	_output/bin/linux/arm/checkpoint \
	_output/bin/linux/arm64/checkpoint \
	_output/bin/linux/ppc64le/checkpoint \
	_output/bin/linux/s390x/checkpoint

release: \
	check \
	cross \
	_output/release/bootkube.tar.gz \

check: gofmt
ifdef TERRAFORM
	$(TERRAFORM) fmt -check ; if [ ! $$? -eq 0 ]; then exit 1; fi
else
	@echo -e "\e[91mSkipping terraform lint. terraform binary not available.\e[0m"
endif
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/\|/e2e')

gofmt:
	gofmt -s -w $(GOFILES)

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/bootkube

release-bootkube: release
	BUILD_IMAGE=bootkube GOARCH=$(GOARCH) IMAGE_REPO=$(IMAGE_REPO_BOOTKUBE) \
	VERSION=$(VERSION) \
		./build/build-image.sh

release-podcheckpointer: release
	BUILD_IMAGE=checkpoint GOARCH=$(GOARCH) IMAGE_REPO=$(IMAGE_REPO_PODCHECKPOINTER) \
	VERSION=$(VERSION) \
		./build/build-image.sh

_output/bin/linux/amd64/%: GOARGS = GOOS=linux GOARCH=amd64
_output/bin/linux/arm/%: GOARGS = GOOS=linux GOARCH=arm
_output/bin/linux/arm64/%: GOARGS = GOOS=linux GOARCH=arm64
_output/bin/linux/ppc64le/%: GOARGS = GOOS=linux GOARCH=ppc64le
_output/bin/linux/s390x/%: GOARGS = GOOS=linux GOARCH=s390x
_output/bin/darwin/amd64/%: GOARGS = GOOS=darwin GOARCH=amd64

_output/bin/%: $(GOFILES) $(VENDOR_GOFILES)
	mkdir -p $(dir $@)
	$(GOARGS) go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ github.com/kubernetes-sigs/bootkube/cmd/$(notdir $@)

_output/release/bootkube.tar.gz: cross
	mkdir -p $(dir $@)
	tar czf $@ -C _output bin

run-%: GOFLAGS = -i
run-%: _output/bin/linux/bootkube _output/bin/$(LOCAL_OS)/bootkube
	@cd hack/$*-node && ./bootkube-up
	@echo "Bootkube ready"

clean-vm-single:
clean-vm-%:
	@echo "Cleaning VM..."
	@(cd hack/$*-node && \
	    vagrant destroy -f && \
	    rm -rf cluster )

#TODO(aaron): Prompt because this is destructive
conformance-%: all
	@cd hack/$*-node && vagrant destroy -f
	@cd hack/$*-node && rm -rf cluster
	@cd hack/$*-node && ./bootkube-up
	@sleep 30 # Give addons a little time to start
	@cd hack/$*-node && ./conformance-test.sh

#TODO: curl/sed "vendored" libs is gross - come up with something better
vendor:
	@go mod vendor
	@curl https://raw.githubusercontent.com/kubernetes/kubernetes/v1.16.2/pkg/kubelet/util/util.go | sed 's/^package util$$/package internal/' > pkg/checkpoint/internal/util.go
	@curl https://raw.githubusercontent.com/kubernetes/kubernetes/v1.16.2/pkg/kubelet/util/util_unix.go | sed 's/^package util$$/package internal/' > pkg/checkpoint/internal/util_unix.go
	@CGO_ENABLED=1 go build -o _output/bin/license-bill-of-materials ./vendor/github.com/coreos/license-bill-of-materials
	@./_output/bin/license-bill-of-materials ./cmd/bootkube ./cmd/checkpoint > bill-of-materials.json

clean:
	rm -rf _output

.PHONY: all check clean gofmt install release vendor
