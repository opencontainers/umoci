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
UMOCI_LINK := vendor/src/$(PROJECT)

# Version information.
VERSION := $(shell cat ./VERSION)
COMMIT_NO := $(shell git rev-parse HEAD 2> /dev/null || true)
COMMIT := $(if $(shell git status --porcelain --untracked-files=no),"${COMMIT_NO}-dirty","${COMMIT_NO}")

.DEFAULT: umoci

umoci: | $(UMOCI_LINK)
	GOPATH=$(PWD)/vendor $(GO) build -i -ldflags "-X main.gitCommit=${COMMIT} -X main.version=${VERSION}" -tags "$(BUILDTAGS)" -o umoci $(PROJECT)/cmd/umoci

$(UMOCI_LINK):
	ln -sfnT . vendor/src
	mkdir -p $(dir $(UMOCI_LINK))
	ln -sfnT $(CURDIR) $(UMOCI_LINK)

clean:
	rm -f umoci
	rm -f vendor/src
	rm -f $(UMOCI_LINK)
