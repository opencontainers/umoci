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
SHELL = /bin/bash

# Go tools.
GO ?= go
GO_MD2MAN ?= go-md2man
export GO111MODULE=on

# Set up the ... lovely ... GOPATH hacks.
PROJECT := github.com/opencontainers/umoci
CMD := ${PROJECT}/cmd/umoci

# We use Docker because Go is just horrific to deal with.
UMOCI_IMAGE := umoci_dev

# TODO: We should test umoci with all of the security options disabled so that
#       we can make sure umoci inside containers works fine (all of these
#       security options are necessary for the test code to run, not umoci
#       itself). The AppArmor/SELinux settings are needed because of the
#       mount-related tests, and the seccomp/systempaths settings are required
#       for the runc tests for rootless containers.
# XXX: Ideally we'd use --security-opt systempaths=unconfined, but the version
#      of Docker in Travis-CI doesn't support that. Bind-mounting the host's
#      proc into the container is more dangerous but has the same effect on the
#      in-kernel mnt_too_revealing() checks and works on old Docker.
DOCKER_RUN          := docker run --rm -it \
                                  -v ${PWD}:/go/src/${PROJECT} \
                                  --security-opt seccomp=unconfined \
                                  --security-opt apparmor=unconfined \
                                  --security-opt label=disable \
                                  -v /proc:/tmp/.HOTFIX-stashed-proc
DOCKER_ROOTPRIV_RUN := $(DOCKER_RUN) --privileged --cap-add=SYS_ADMIN
DOCKER_ROOTLESS_RUN := $(DOCKER_RUN) -u 1000:1000 --cap-drop=all

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
BASE_LDFLAGS := -s -w -X main.gitCommit=${COMMIT} -X main.version=${VERSION}

# Specific build flags for build type.
DYN_BUILD_FLAGS := ${BASE_FLAGS} -buildmode=pie -ldflags "${BASE_LDFLAGS}"
TEST_BUILD_FLAGS := ${BASE_FLAGS} -buildmode=pie -ldflags "${BASE_LDFLAGS} -X ${PROJECT}/pkg/testutils.binaryType=test"
STATIC_BUILD_FLAGS := ${BASE_FLAGS} -ldflags "${BASE_LDFLAGS} -extldflags '-static'"

# Installation directories.
DESTDIR ?=
PREFIX ?=/usr
BINDIR ?=$(PREFIX)/bin
MANDIR ?=$(PREFIX)/share/man

.DEFAULT: umoci

GO_SRC = $(shell find . -type f -name '*.go')

# NOTE: If you change these make sure you also update local-validate-build.

umoci: $(GO_SRC)
	$(GO) build ${DYN_BUILD_FLAGS} -o $(BUILD_DIR)/$@ ${CMD}

umoci.static: $(GO_SRC)
	env CGO_ENABLED=0 $(GO) build ${STATIC_BUILD_FLAGS} -o $(BUILD_DIR)/$@ ${CMD}

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
	rm -f umoci umoci.static umoci.cov*
	rm -f $(MANPAGES)

.PHONY: validate
validate: umociimage
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
DOCKER_IMAGE :=opensuse/amd64:tumbleweed

.PHONY: umociimage
umociimage:
	docker build -t $(UMOCI_IMAGE) --build-arg DOCKER_IMAGE=$(DOCKER_IMAGE) .

ifndef COVERAGE
COVERAGE := $(notdir $(shell mktemp -u umoci.cov.XXXXXX))
export COVERAGE
endif

.PHONY: test-unit
test-unit: umociimage
	touch $(COVERAGE) && chmod a+rw $(COVERAGE)
	$(DOCKER_ROOTPRIV_RUN) -e COVERAGE=$(COVERAGE) $(UMOCI_IMAGE) make local-test-unit
	$(DOCKER_ROOTLESS_RUN) -e COVERAGE=$(COVERAGE) $(UMOCI_IMAGE) make local-test-unit

.PHONY: local-test-unit
local-test-unit:
	GO=$(GO) COVER=1 hack/test-unit.sh

.PHONY: test-integration
test-integration: umociimage
	touch $(COVERAGE) && chmod a+rw $(COVERAGE)
	$(DOCKER_ROOTPRIV_RUN) -e COVERAGE=$(COVERAGE) $(UMOCI_IMAGE) make local-test-integration
	$(DOCKER_ROOTLESS_RUN) -e COVERAGE=$(COVERAGE) $(UMOCI_IMAGE) make local-test-integration

.PHONY: local-test-integration
local-test-integration: umoci.cover
	TESTS="${TESTS}" COVER=1 hack/test-integration.sh

.PHONY: shell
shell: umociimage
	$(DOCKER_RUN) $(UMOCI_IMAGE) bash

.PHONY: root-shell
root-shell: umociimage
	$(DOCKER_ROOTPRIV_RUN) $(UMOCI_IMAGE) bash

.PHONY: rootless-shell
rootless-shell: umociimage
	$(DOCKER_ROOTLESS_RUN) $(UMOCI_IMAGE) bash

.PHONY: ci
ci: umoci umoci.cover validate docs test-unit test-integration
	hack/ci-coverage.sh $(COVERAGE)
