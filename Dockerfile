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

FROM registry.opensuse.org/opensuse/leap:42.3
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
		git \
		gnu_parallel \
		"go>=1.18" \
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
RUN go install github.com/cpuguy83/go-md2man/v2@latest && \
	go install golang.org/x/lint/golint@latest && \
	go install github.com/securego/gosec/cmd/gosec@latest && \
	go install github.com/client9/misspell/cmd/misspell@latest
# FIXME: We need to get an ancient version of oci-runtime-tools because the
#        config.json conversion we do is technically not spec-compliant due to
#        an oversight and new versions of oci-runtime-tools verify this.
#        See <https://github.com/opencontainers/runtime-spec/pull/1197>.
RUN go install github.com/opencontainers/runtime-tools/cmd/oci-runtime-tool@v0.5.0
# FIXME: oci-image-tool was basically broken for our needs after v0.3.0 (it
#        cannot scan image layouts). The source is so old we need to manually
#        build it (including doing "go mod init").
RUN git clone -b v0.3.0 https://github.com/opencontainers/image-tools.git /tmp/oci-image-tools && \
	( cd /tmp/oci-image-tools && go mod init github.com/opencontainers/image-tools && go mod tidy && go mod vendor; ) && \
	make -C /tmp/oci-image-tools all install && \
	rm -rf /tmp/oci-image-tools

ENV SOURCE_IMAGE=/opensuse SOURCE_TAG=latest
ARG TEST_DOCKER_IMAGE=registry.opensuse.org/opensuse/leap:15.4
RUN skopeo copy docker://$TEST_DOCKER_IMAGE oci:$SOURCE_IMAGE:$SOURCE_TAG

VOLUME ["/go/src/github.com/opencontainers/umoci"]
WORKDIR /go/src/github.com/opencontainers/umoci
