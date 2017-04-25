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

release: \
	clean \
	check \
	_output/release/bootkube.tar.gz \

check:
	@find . -name vendor -prune -o -name '*.go' -exec gofmt -s -d {} +
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@go test -v $(shell go list ./... | grep -v '/vendor/')

install: _output/bin/$(LOCAL_OS)/bootkube
	cp $< $(GOPATH_BIN)

_output/bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	GOOS=$(word 1, $(subst /, ,$*)) go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ github.com/kubernetes-incubator/bootkube/cmd/$(notdir $@)

_output/release/bootkube.tar.gz: _output/bin/linux/bootkube _output/bin/darwin/bootkube _output/bin/linux/checkpoint
	mkdir -p $(dir $@)
	tar czf $@ -C _output bin/linux/bootkube bin/darwin/bootkube bin/linux/checkpoint

run-%: GOFLAGS = -i
run-%: clean-vm-% _output/bin/linux/bootkube _output/bin/$(LOCAL_OS)/bootkube
	@cd hack/$*-node && ./bootkube-up
	@echo "Bootkube ready"

clean-vm-single:
clean-vm-%:
	@echo "Cleaning VM..."
	@(cd hack/$*-node && \
	    vagrant destroy -f && \
	    rm -rf cluster )

#TODO(aaron): Prompt because this is destructive
conformance-%: clean all
	@cd hack/$*-node && vagrant destroy -f
	@cd hack/$*-node && rm -rf cluster
	@cd hack/$*-node && ./bootkube-up
	@sleep 30 # Give addons a little time to start
	@cd hack/$*-node && ./conformance-test.sh

vendor:
	@glide update --delete --update-vendored --strip-vcs --strip-vendor
	@glide-vc

clean:
	rm -rf _output

.PHONY: all check clean install release vendor
