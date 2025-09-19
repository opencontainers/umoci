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

## TOOLS: Basic golang tools can be installed using standard "go install".
FROM golang:1.25 AS go-binaries
ENV GOPATH=/go PATH=/go/bin:$PATH
RUN go install github.com/cpuguy83/go-md2man/v2@latest

## TOOLS: oci-runtime-tool needs special handling.
FROM golang:1.25 AS oci-runtime-tool
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
ENV SRCDIR=/tmp/oci-runtime-tool
RUN git clone -b v0.5.0 https://github.com/opencontainers/runtime-tools.git $SRCDIR
RUN cd $SRCDIR && \
	go mod init github.com/opencontainers/runtime-tools && \
	go mod tidy && \
	go get github.com/opencontainers/runtime-spec@v1.0.2 && \
	go mod vendor
RUN make -C $SRCDIR tool
RUN install -Dm 0755 $SRCDIR/oci-runtime-tool /usr/bin/oci-runtime-tool

## CI: Pull the test image in a separate build stage.
FROM quay.io/skopeo/stable:v1.20 AS test-image
ENV SOURCE_IMAGE=/image SOURCE_TAG=latest
ARG TEST_DOCKER_IMAGE=registry.opensuse.org/opensuse/tumbleweed:latest
RUN skopeo copy docker://$TEST_DOCKER_IMAGE oci:$SOURCE_IMAGE:$SOURCE_TAG

## CI: Final stage, putting together the image used for our actual tests.
FROM registry.opensuse.org/opensuse/leap:16.0 AS ci-image
LABEL org.opencontainers.image.authors="Aleksa Sarai <cyphar@cyphar.com>"

RUN zypper ar -f -p 10 -g 'obs://home:cyphar:containers/$releasever' obs-gomtree && \
	zypper --gpg-auto-import-keys -n ref
RUN zypper -n up
RUN zypper -n in \
		attr \
		bats \
		bc \
		curl \
		diff \
		file \
		findutils \
		git \
		gnu_parallel \
		# Go 1.25's packaging is broken at the moment.
		# See <https://bugzilla.suse.com/show_bug.cgi?id=1249985>.
		go1.24 \
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
RUN git config --system --add safe.directory /go/src/github.com/opencontainers/umoci

ENV GOPATH=/go PATH=/go/bin:$PATH
COPY --from=go-binaries /go/bin /go/bin
COPY --from=oci-runtime-tool /usr/bin/oci-runtime-tool /go/bin
ENV SOURCE_IMAGE=/image SOURCE_TAG=latest
COPY --from=test-image $SOURCE_IMAGE $SOURCE_IMAGE

VOLUME ["/go/src/github.com/opencontainers/umoci"]
WORKDIR /go/src/github.com/opencontainers/umoci
