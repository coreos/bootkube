export CGO_ENABLED:=0
export GOARCH:=amd64
export PATH:=$(PATH):$(PWD)

GOOS ?= $(shell go env GOOS)
GOFILES:=$(shell find . -name '*.go' ! -path './vendor/*')
LDFLAGS=-X github.com/kubernetes-incubator/bootkube/pkg/version.Version=$(shell $(CURDIR)/build/git-version.sh)
TERRAFORM:=$(shell command -v terraform 2> /dev/null)

all: \
	_output/bin/$(GOOS)/bootkube \
	_output/bin/linux/bootkube \
	_output/bin/linux/checkpoint

cross: \
	_output/bin/linux/bootkube \
	_output/bin/darwin/bootkube \
	_output/bin/linux/checkpoint \
	_output/bin/linux/amd64/checkpoint \
	_output/bin/linux/arm/checkpoint \
	_output/bin/linux/arm64/checkpoint \
	_output/bin/linux/ppc64le/checkpoint \
	_output/bin/linux/s390x/checkpoint

release: \
	clean \
	check \
	_output/release/bootkube.tar.gz \

check: gofmt
ifdef TERRAFORM
	$(TERRAFORM) fmt -check ; if [ ! $$? -eq 0 ]; then exit 1; fi
else
	@echo -e "\e[91mSkipping terraform lint. terraform binary not available.\e[0m"
endif
	@go vet $(shell go list ./... | grep -v '/vendor/')
	@./scripts/verify-gopkg.sh
	@go test -v $(shell go list ./... | grep -v '/vendor/\|/e2e')

gofmt:
	gofmt -s -w $(GOFILES)
	git diff --exit-code

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/bootkube

_output/bin/%: GOOS=$(word 1, $(subst /, ,$*))
_output/bin/%: GOARCH=$(word 2, $(subst /, ,$*))
_output/bin/%: GOARCH:=amd64  # default to amd64 to support release scripts
_output/bin/%: $(GOFILES)
	mkdir -p $(dir $@)
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $@ github.com/kubernetes-incubator/bootkube/cmd/$(notdir $@)

_output/release/bootkube.tar.gz: _output/bin/linux/bootkube _output/bin/darwin/bootkube _output/bin/linux/checkpoint
	mkdir -p $(dir $@)
	tar czf $@ -C _output bin/linux/bootkube bin/darwin/bootkube bin/linux/checkpoint

run-%: GOFLAGS = -i
run-%: clean-vm-% _output/bin/linux/bootkube _output/bin/$(GOOS)/bootkube
	test $(GOOS) = $$(uname | tr A-Z a-z)
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

#TODO: curl/sed "vendored" libs is gross - come up with something better
vendor:
	@dep ensure -v
	@curl https://raw.githubusercontent.com/kubernetes/kubernetes/v1.11.2/pkg/kubelet/util/util.go | sed 's/^package util$$/package internal/' > pkg/checkpoint/internal/util.go
	@curl https://raw.githubusercontent.com/kubernetes/kubernetes/v1.11.2/pkg/kubelet/util/util_unix.go | sed 's/^package util$$/package internal/' > pkg/checkpoint/internal/util_unix.go
	@CGO_ENABLED=1 go build -o _output/bin/license-bill-of-materials ./vendor/github.com/coreos/license-bill-of-materials
	@./_output/bin/license-bill-of-materials ./cmd/bootkube ./cmd/checkpoint > bill-of-materials.json

clean:
	rm -rf _output

.PHONY: all check clean gofmt install release vendor
