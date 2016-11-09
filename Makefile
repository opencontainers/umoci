# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016 SUSE LLC.
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

GO ?= go

# Set up the ... lovely ... GOPATH hacks.
PROJECT := github.com/cyphar/umoci
UMOCI_LINK := vendor/$(PROJECT)

# Version information.
VERSION := $(shell cat ./VERSION)
COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
COMMIT := $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")

.DEFAULT: umoci

GO_SRC =  $(shell find . -name \*.go)
umoci: $(GO_SRC)
	$(GO) build -i -ldflags "-X main.gitCommit=${COMMIT} -X main.version=${VERSION}" -tags "$(BUILDTAGS)" -o umoci $(PROJECT)/cmd/umoci

update-deps:
	hack/vendor.sh

clean:
	rm -f umoci

install-deps:
	go get -u github.com/golang/lint/golint
	go get -u github.com/vbatts/git-validation

EPOCH_COMMIT ?= 97ecdbd53dcb72b7a0d62196df281f131dc9eb2f
validate:
	@echo "go-fmt"
	@test -z "$$(gofmt -s -l . | grep -v '^vendor/' | grep -v '^third_party/' | tee /dev/stderr)"
	@echo "go-lint"
	@out="$$(golint $(PROJECT)/... | grep -v '/vendor/' | grep -v '/third_party/')"; \
	if [ -n "$$out" ]; then \
		echo "$$out"; \
		exit 1; \
	fi
	@echo "go-vet"
	@go vet $(shell go list $(PROJECT)/... | grep -v /vendor/ | grep -v /third_party/)
	#@echo "git-validation"
	#@git-validation -v -run DCO,short-subject,dangling-whitespace $(EPOCH_COMMIT)..HEAD

test:
	go test $(PROJECT)/...

ci: umoci validate test
