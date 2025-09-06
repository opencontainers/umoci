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

FROM registry.opensuse.org/opensuse/leap:15.6
MAINTAINER "Aleksa Sarai <asarai@suse.com>"

# We have to use out-of-tree repos because several packages haven't been merged
# into openSUSE Leap yet, or are out of date in Leap.
RUN zypper mr -d repo-non-oss repo-update-non-oss && \
	zypper ar -f -p 10 -g 'obs://Virtualization:containers/$releasever' obs-vc && \
	zypper ar -f -p 10 -g 'obs://devel:tools/$releasever'               obs-tools && \
	zypper ar -f -p 10 -g 'obs://devel:languages:go/$releasever'        obs-go && \
	zypper ar -f -p 10 -g 'obs://home:cyphar:containers/$releasever'    obs-gomtree && \
	zypper --gpg-auto-import-keys -n ref && \
	zypper -n up
RUN zypper -n in \
		attr \
		bats \
		bc \
		curl \
		file \
		git \
		gnu_parallel \
		"go>=1.23" \
		go-mtree \
		gzip \
		jq \
		libcap-progs \
		make \
		moreutils \
		python3-xattr python3-setuptools \
		runc \
		skopeo \
		tar \
		which
RUN useradd -u 1000 -m -d /home/rootless -s /bin/bash rootless

ENV GOPATH=/go PATH=/go/bin:$PATH
RUN go install github.com/cpuguy83/go-md2man/v2@latest

# FIXME: We need to get an ancient version of oci-runtime-tools because the
#        config.json conversion we do is technically not spec-compliant due to
#        an oversight and new versions of oci-runtime-tools verify this.
#        See <https://github.com/opencontainers/runtime-spec/pull/1197>.
#
#        In addition, there is no go.mod in all released versions up to v0.9.0,
#        which means that we will pull the latest runtime-spec automatically
#        (Go removed auto-conversion to go.mod in Go 1.22) which causes
#        validation errors. But we need to forcefully pick runtime-spec v1.0.2.
#        This is fine. See <https://github.com/opencontainers/runtime-tools/pull/774>.
RUN git clone -b v0.5.0 https://github.com/opencontainers/runtime-tools.git /tmp/oci-runtime-tools && \
	( cd /tmp/oci-runtime-tools && \
		go mod init github.com/opencontainers/runtime-tools && \
		go mod tidy && \
		go get github.com/opencontainers/runtime-spec@v1.0.2 && \
		go mod vendor; ) && \
	make -C /tmp/oci-runtime-tools tool install && \
	rm -rf /tmp/oci-runtime-tools

ENV SOURCE_IMAGE=/opensuse SOURCE_TAG=latest
ARG TEST_DOCKER_IMAGE=registry.opensuse.org/opensuse/leap:15.4
RUN skopeo copy docker://$TEST_DOCKER_IMAGE oci:$SOURCE_IMAGE:$SOURCE_TAG

RUN git config --system --add safe.directory /go/src/github.com/opencontainers/umoci
VOLUME ["/go/src/github.com/opencontainers/umoci"]
WORKDIR /go/src/github.com/opencontainers/umoci
