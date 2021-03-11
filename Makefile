# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2020 SUSE LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Use bash, so that we can do process substitution.
SHELL := $(shell which bash)

# Go tools.
GO ?= go
GO_MD2MAN ?= go-md2man
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
export GO111MODULE=on

# Set up the ... lovely ... GOPATH hacks.
PROJECT := github.com/opencontainers/umoci
CMD := ${PROJECT}/cmd/umoci

# We use Docker because Go is just horrific to deal with.
UMOCI_IMAGE := umoci/ci:latest

# TODO: We should test umoci with all of the security options disabled so that
#       we can make sure umoci inside containers works fine (all of these
#       security options are necessary for the test code to run, not umoci
#       itself). The AppArmor/SELinux settings are needed because of the
#       mount-related tests, and the seccomp/systempaths settings are required
#       for the runc tests for rootless containers.
DOCKER_RUN = docker run --rm -v ${PWD}:/go/src/${PROJECT} \
                        --security-opt apparmor=unconfined \
                        --security-opt label=disable \
                        --security-opt seccomp=unconfined \
                        --security-opt systempaths=unconfined

# We only add the CodeCov environment (and ping codecov) if we're running in
# Travis, to avoid pinging third-party servers for local builds.
ifdef TRAVIS
$(shell echo "WARNING: This make invocation will fetch and run code from https://codecov.io/." >&2)
DOCKER_RUN += $(shell echo "+ curl -sSL https://codecov.io/env | bash" >&2) \
              $(shell ./hack/resilient-curl.sh -sSL https://codecov.io/env | bash)
endif

DOCKER_ROOTPRIV_RUN = $(DOCKER_RUN) --privileged --cap-add=SYS_ADMIN
DOCKER_ROOTLESS_RUN = $(DOCKER_RUN) -u 1000:1000 --cap-drop=all

# Output directory.
BUILD_DIR ?= .

# Release information.
GPG_KEYID ?=

# Version information.
VERSION := $(shell cat ./VERSION)
COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
COMMIT := $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")

# Basic build flags.
BUILD_FLAGS ?=
BASE_FLAGS := ${BUILD_FLAGS} -tags "${BUILDTAGS}"
BASE_LDFLAGS := -s -w -X ${PROJECT}.gitCommit=${COMMIT} -X ${PROJECT}.version=${VERSION}

# Specific build flags for build type.
ifeq ($(GOOS), linux)
	TEST_BUILD_FLAGS := ${BASE_FLAGS} -buildmode=pie -ldflags "${BASE_LDFLAGS} -X ${PROJECT}/pkg/testutils.binaryType=test"		   DYN_BUILD_FLAGS := ${BASE_FLAGS} -buildmode=pie -ldflags "${BASE_LDFLAGS}"
	TEST_BUILD_FLAGS := ${BASE_FLAGS} -buildmode=pie -ldflags "${BASE_LDFLAGS} -X ${PROJECT}/pkg/testutils.binaryType=test"
else
	DYN_BUILD_FLAGS := ${BASE_FLAGS} -ldflags "${BASE_LDFLAGS}"
	TEST_BUILD_FLAGS := ${BASE_FLAGS} -ldflags "${BASE_LDFLAGS} -X ${PROJECT}/pkg/testutils.binaryType=test"
endif


STATIC_BUILD_FLAGS := ${BASE_FLAGS} -ldflags "${BASE_LDFLAGS} -extldflags '-static'"

# Installation directories.
DESTDIR ?=
PREFIX ?=/usr/local
BINDIR ?=$(PREFIX)/bin
MANDIR ?=$(PREFIX)/share/man

.DEFAULT: umoci

GO_SRC = $(shell find . -type f -name '*.go')

# NOTE: If you change these make sure you also update local-validate-build.

umoci: $(GO_SRC)
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build ${DYN_BUILD_FLAGS} -o $(BUILD_DIR)/$@ ${CMD}

umoci.static: $(GO_SRC)
	env CGO_ENABLED=0 $(GO) build ${STATIC_BUILD_FLAGS} -o $(BUILD_DIR)/$@ ${CMD}

.PHONY: static
static: umoci.static

umoci.cover: $(GO_SRC)
	$(GO) test -c -cover -covermode=count -coverpkg=./... ${TEST_BUILD_FLAGS} -o $(BUILD_DIR)/$@ ${CMD}

.PHONY: release
release:
	hack/release.sh -v $(VERSION) -S "$(GPG_KEYID)"

.PHONY: install
install: umoci docs
	install -D -m0755 umoci $(DESTDIR)/$(BINDIR)/umoci
	-for man in $(MANPAGES); do \
		filename="$$(basename -- "$$man")"; \
		target="$(DESTDIR)/$(MANDIR)/man$${filename##*.}/$$filename"; \
		install -D -m0644 "$$man" "$$target"; \
		gzip -9f "$$target"; \
	 done

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)/$(BINDIR)/umoci
	-rm -f $(DESTDIR)/$(MANDIR)/man*/umoci*

.PHONY: clean
clean:
	rm -f umoci umoci.static umoci-ci.tar umoci.cov*
	rm -f $(MANPAGES)

.PHONY: validate
validate: ci-image
	$(DOCKER_RUN) $(UMOCI_IMAGE) make local-validate

.PHONY: local-validate
local-validate: local-validate-go local-validate-spell local-validate-reproducible local-validate-build

.PHONY: local-validate-go
local-validate-go:
	@type gofmt     >/dev/null 2>/dev/null || (echo "ERROR: gofmt not found." && false)
	test -z "$$(gofmt -s -l . | grep -vE '^vendor/|^third_party/' | tee /dev/stderr)"
	@type golint    >/dev/null 2>/dev/null || (echo "ERROR: golint not found." && false)
	test -z "$$(golint $(PROJECT)/... | grep -vE '/vendor/|/third_party/' | tee /dev/stderr)"
	@go doc cmd/vet >/dev/null 2>/dev/null || (echo "ERROR: go vet not found." && false)
	test -z "$$($(GO) vet $$($(GO) list $(PROJECT)/... | grep -vE '/vendor/|/third_party/') 2>&1 | tee /dev/stderr)"
	@type gosec     >/dev/null 2>/dev/null || (echo "ERROR: gosec not found." && false)
	test -z "$$(gosec -quiet -exclude=G301,G302,G304 $$GOPATH/$(PROJECT)/... | tee /dev/stderr)"
	./hack/test-vendor.sh

.PHONY: local-validate-spell
local-validate-spell:
	make clean
	@type misspell  >/dev/null 2>/dev/null || (echo "ERROR: misspell not found." && false)
	test -z "$$(find . -type f -print0 | xargs -0 misspell | grep -vE '/(vendor|third_party|\.site)/' | tee /dev/stderr)"

# Make sure that our builds are reproducible even if you wait between them and
# the modified time of the files is different.
.PHONY: local-validate-reproducible
local-validate-reproducible:
	mkdir -p .tmp-validate
	make -B umoci && cp umoci .tmp-validate/umoci.a
	@echo sleep 10s
	@sleep 10s && touch $(GO_SRC)
	make -B umoci && cp umoci .tmp-validate/umoci.b
	diff -s .tmp-validate/umoci.{a,b}
	sha256sum .tmp-validate/umoci.{a,b}
	rm -r .tmp-validate/umoci.{a,b}

.PHONY: local-validate-build
local-validate-build:
	$(GO) build ${DYN_BUILD_FLAGS} -o /dev/null ${CMD}
	env CGO_ENABLED=0 $(GO) build ${STATIC_BUILD_FLAGS} -o /dev/null ${CMD}
	$(GO) test -run nothing ${DYN_BUILD_FLAGS} $(PROJECT)/...

MANPAGES_MD := $(wildcard doc/man/*.md)
MANPAGES    := $(MANPAGES_MD:%.md=%)

doc/man/%.1: doc/man/%.1.md
	$(GO_MD2MAN) -in $< -out $@

.PHONY: docs
docs: $(MANPAGES)

# Used for tests.
DOCKER_IMAGE ?=registry.opensuse.org/opensuse/leap:latest

ifndef COVERAGE
COVERAGE := $(notdir $(shell mktemp -u umoci.cov.XXXXXX))
endif
export COVERAGE

.PHONY: test-unit
test-unit: ci-image
	touch $(COVERAGE) && chmod a+rw $(COVERAGE)
	$(DOCKER_ROOTPRIV_RUN) -e COVERAGE $(UMOCI_IMAGE) make local-test-unit
	$(DOCKER_ROOTLESS_RUN) -e COVERAGE $(UMOCI_IMAGE) make local-test-unit

.PHONY: local-test-unit
local-test-unit:
	GO=$(GO) hack/test-unit.sh

.PHONY: test-integration
test-integration: ci-image
	touch $(COVERAGE) && chmod a+rw $(COVERAGE)
	$(DOCKER_ROOTPRIV_RUN) -e COVERAGE -e TESTS $(UMOCI_IMAGE) make local-test-integration
	$(DOCKER_ROOTLESS_RUN) -e COVERAGE -e TESTS $(UMOCI_IMAGE) make local-test-integration

.PHONY: local-test-integration
local-test-integration: umoci.cover
	TESTS="${TESTS}" hack/test-integration.sh

.PHONY: shell
shell: ci-image
	$(DOCKER_RUN) -it $(UMOCI_IMAGE) bash

.PHONY: root-shell
root-shell: ci-image
	$(DOCKER_ROOTPRIV_RUN) -it $(UMOCI_IMAGE) bash

.PHONY: rootless-shell
rootless-shell: ci-image
	$(DOCKER_ROOTLESS_RUN) -it $(UMOCI_IMAGE) bash

CACHE := .cache
CACHE_IMAGE := $(CACHE)/ci-image.tar.zst

.PHONY: ci-image
ci-image:
	docker pull registry.opensuse.org/opensuse/leap:latest
	! [ -f "$(CACHE_IMAGE)" ] || unzstd < "$(CACHE_IMAGE)" | docker load
	DOCKER_BUILDKIT=1 docker build -t $(UMOCI_IMAGE) \
	                               --progress plain \
	                               --cache-from $(UMOCI_IMAGE) \
	                               --build-arg DOCKER_IMAGE=$(DOCKER_IMAGE) \
	                               --build-arg BUILDKIT_INLINE_CACHE=1 .

.PHONY: ci-cache
ci-cache: ci-image
	rm -rf $(CACHE) && mkdir -p $(CACHE)
	docker save $(UMOCI_IMAGE) | zstd > $(CACHE_IMAGE)

.PHONY: ci-validate
ci-validate: umoci umoci.static
	make docs local-validate

.PHONY: ci-unit
ci-unit: umoci.cover
	make test-unit

.PHONY: ci-integration
ci-integration: umoci.cover
	make test-integration

.PHONY: ci
ci:
	@echo "NOTE: This is not identical to the upstream CI, but the tests are the same."
	make ci-validate ci-unit ci-integration
	hack/ci-coverage.sh $(COVERAGE)
