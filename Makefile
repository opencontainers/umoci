# SPDX-License-Identifier: Apache-2.0
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2025 SUSE LLC
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
SHELL := bash

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
BASE_FLAGS := ${BUILD_FLAGS} -tags "${BUILDTAGS}" -buildvcs=false
BASE_LDFLAGS := -s -w -X ${PROJECT}.gitCommit=${COMMIT}

# Specific build flags for build type.
ifeq ($(GOOS), linux)
	DYN_BUILD_FLAGS := ${BASE_FLAGS} -buildmode=pie -ldflags "${BASE_LDFLAGS}"
	TEST_BUILD_FLAGS := ${BASE_FLAGS} -buildmode=pie -ldflags "${BASE_LDFLAGS} -X ${PROJECT}/internal/testhelpers.binaryType=test"
else
	DYN_BUILD_FLAGS := ${BASE_FLAGS} -ldflags "${BASE_LDFLAGS}"
	TEST_BUILD_FLAGS := ${BASE_FLAGS} -ldflags "${BASE_LDFLAGS} -X ${PROJECT}/internal/testhelpers.binaryType=test"
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
	$(GO) build ${TEST_BUILD_FLAGS} -cover -covermode=count -coverpkg=./... -o $(BUILD_DIR)/$@ ${CMD}

.PHONY: release
release:
	hack/release.sh \
		-a 386 -a amd64 -a arm64 -a ppc64le -a riscv64 -a s390x \
		-v $(VERSION) -S "$(GPG_KEYID)"

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
local-validate: local-validate-go local-validate-reproducible local-validate-build

.PHONY: local-validate-keyring
local-validate-keyring:
	./hack/keyring_validate.sh

.PHONY: local-validate-go
local-validate-go:
	@type golangci-lint >/dev/null 2>/dev/null || (echo "ERROR: golanglint-ci not found." && false)
	golangci-lint run --max-issues-per-linter 0 --max-same-issues 0
	./hack/test-vendor.sh

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

ifndef GOCOVERDIR
GOCOVERDIR := $(notdir $(shell mktemp -d -u umoci.cov.XXXXXX))
endif
export GOCOVERDIR

.PHONY: test-unit
test-unit: ci-image
	mkdir -p $(GOCOVERDIR) && chmod a+rwx $(GOCOVERDIR)
	$(DOCKER_ROOTPRIV_RUN) -e GOCOVERDIR $(UMOCI_IMAGE) make local-test-unit
	$(DOCKER_ROOTLESS_RUN) -e GOCOVERDIR $(UMOCI_IMAGE) make local-test-unit

.PHONY: local-test-unit
local-test-unit:
	GO=$(GO) hack/test-unit.sh

TESTS ?=

.PHONY: test-integration
test-integration: test-root-integration test-rootless-integration

.PHONY: test-root-integration
test-root-integration: ci-image umoci.cover
	mkdir -p $(GOCOVERDIR) && chmod a+rwx $(GOCOVERDIR)
	$(DOCKER_ROOTPRIV_RUN) -e GOCOVERDIR -e TESTS $(UMOCI_IMAGE) make local-test-integration

.PHONY: test-rootless-integration
test-rootless-integration: ci-image umoci.cover
	mkdir -p $(GOCOVERDIR) && chmod a+rwx $(GOCOVERDIR)
	$(DOCKER_ROOTLESS_RUN) -e GOCOVERDIR -e TESTS $(UMOCI_IMAGE) make local-test-integration

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

TEST_DOCKER_IMAGE ?=$(shell sed -En 's/^ARG\s+TEST_DOCKER_IMAGE=([^ ]*).*$$/\1/p' Dockerfile)
CI_CACHE_PATH ?=.ci-cache

.PHONY: ci-cache
ci-cache: BUILDX_CACHE := \
	--cache-from=type=local,src=$(CI_CACHE_PATH) \
	--cache-to=type=local,dest=$(CI_CACHE_PATH)
ci-cache: ci-image

.PHONY: ci-image
ci-image:
	docker buildx build $(BUILDX_CACHE) \
		-o type=docker -t $(UMOCI_IMAGE) \
		--pull \
		--build-arg TEST_DOCKER_IMAGE=$(TEST_DOCKER_IMAGE) .

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
	hack/ci-coverage.sh --func $(GOCOVERDIR)
