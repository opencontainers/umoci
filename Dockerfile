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

FROM registry.opensuse.org/opensuse/leap:latest
MAINTAINER "Aleksa Sarai <asarai@suse.com>"

# We have to use out-of-tree repos because several packages haven't been merged
# into openSUSE Leap yet, or are out of date in Leap.
RUN zypper mr -d repo-non-oss repo-update-non-oss && \
	zypper ar -f -p 5 -g obs://home:cyphar:bats obs-bats && \
	zypper ar -f -p 10 -g obs://Virtualization:containers obs-vc && \
	zypper ar -f -p 10 -g obs://devel:tools obs-tools && \
	zypper ar -f -p 10 -g obs://devel:languages:go obs-go && \
	zypper --gpg-auto-import-keys -n ref && \
	zypper -n up
RUN zypper -n in \
		attr \
		bats \
		bc \
		curl \
		git \
		gnu_parallel \
		go1.14 \
		go-mtree \
		gzip \
		jq \
		libcap-progs \
		make \
		moreutils \
		oci-image-tools \
		oci-runtime-tools \
		python-setuptools \
		python-xattr \
		runc \
		skopeo \
		tar \
		which
RUN useradd -u 1000 -m -d /home/rootless -s /bin/bash rootless

ENV GOPATH=/go PATH=/go/bin:$PATH
RUN go get -u github.com/cpuguy83/go-md2man && \
    go get -u golang.org/x/lint/golint && \
    go get -u github.com/securego/gosec/cmd/gosec && \
    go get -u github.com/client9/misspell/cmd/misspell

ENV SOURCE_IMAGE=/opensuse SOURCE_TAG=latest
ARG DOCKER_IMAGE=opensuse/amd64:tumbleweed
RUN skopeo copy docker://$DOCKER_IMAGE oci:$SOURCE_IMAGE:$SOURCE_TAG

VOLUME ["/go/src/github.com/opencontainers/umoci"]
WORKDIR /go/src/github.com/opencontainers/umoci
