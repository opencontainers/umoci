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
# TODO: Get <https://github.com/vbatts/go-mtree/pull/211>,
#       <https://github.com/vbatts/go-mtree/pull/212>, and
#       <https://github.com/vbatts/go-mtree/pull/214> merged and switch.
#RUN go install github.com/vbatts/go-mtree@latest
RUN git clone -b umoci https://github.com/cyphar/go-mtree.git /tmp/gomtree
RUN cd /tmp/gomtree && \
	go install ./cmd/gomtree

## CI: Pull the test image in a separate build stage.
FROM quay.io/skopeo/stable:v1.21 AS test-image
ENV SOURCE_IMAGE=/image SOURCE_TAG=latest
ARG TEST_DOCKER_IMAGE=registry.opensuse.org/opensuse/tumbleweed:latest
RUN skopeo copy docker://$TEST_DOCKER_IMAGE oci:$SOURCE_IMAGE:$SOURCE_TAG

## CI: Final stage, putting together the image used for our actual tests.
FROM registry.opensuse.org/opensuse/leap:16.0 AS ci-image
LABEL org.opencontainers.image.authors="Aleksa Sarai <cyphar@cyphar.com>"

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
ENV SOURCE_IMAGE=/image SOURCE_TAG=latest
COPY --from=test-image $SOURCE_IMAGE $SOURCE_IMAGE

VOLUME ["/go/src/github.com/opencontainers/umoci"]
WORKDIR /go/src/github.com/opencontainers/umoci
