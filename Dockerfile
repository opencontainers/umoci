# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016, 2017 SUSE LLC.
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

FROM opensuse/amd64:42.2
MAINTAINER "Aleksa Sarai <asarai@suse.com>"

# We have to use out-of-tree repos because several packages haven't been merged
# into openSUSE:Factory yet.
RUN zypper ar -f -p 10 -g obs://Virtualization:containers obs-vc && \
    zypper ar -f -p 10 -g obs://devel:languages:go obs-dlg && \
    zypper ar -f -p 10 -g obs://home:cyphar:containers obs-home-cyphar-containers && \
    zypper ar -f -p 15 -g obs://home:cyphar obs-home-cyphar && \
    zypper ar -f -p 15 -g obs://devel:languages:python3 obs-py3k && \
	zypper --gpg-auto-import-keys -n ref && \
	zypper -n up
RUN zypper -n in \
		bats \
		git \
		'go>=1.6' \
		golang-github-cpuguy83-go-md2man \
		go-mtree \
		jq \
		make \
		moreutils \
		oci-image-tools \
		oci-runtime-tools \
		python3-setuptools \
		python3-xattr \
		skopeo

ENV GOPATH /go
ENV PATH $GOPATH/bin:$PATH
RUN go get -u github.com/golang/lint/golint

# XXX(cyphar): Improve all of the following re-install code in the future. It's
# quite gross IMO, as it means that our packages aren't being tested and we're
# encoding information about external projects in our project.

# Reinstall skopeo from source, since there's a bootstrapping issue because
# packaging of skopeo in openSUSE is often blocked by umoci updates (since KIWI
# uses both). This should no longer be necessary once we hit OCI v1.0.
# NOTE: We can't use 0.1.22 because of libostree.
ENV SKOPEO_VERSION=91e801b45115580c0709905719ae14c42f201027 SKOPEO_PROJECT=github.com/projectatomic/skopeo
RUN zypper -n in \
		device-mapper-devel \
		glib2-devel \
		libbtrfs-devel \
		libgpgme-devel && \
	mkdir -p /go/src/$SKOPEO_PROJECT && \
	git clone https://$SKOPEO_PROJECT /go/src/$SKOPEO_PROJECT && \
	( cd /go/src/$SKOPEO_PROJECT ; git checkout $SKOPEO_VERSION ; ) && \
	make BUILDTAGS="containers_image_ostree_stub" -C /go/src/$SKOPEO_PROJECT binary-local install-binary && \
	rm -rf /go/src/$SKOPEO_PROJECT

# Reinstall oci-image-tools from source to avoid having to package new versions
# in openSUSE while testing PRs.
ENV IMAGETOOLS_VERSION=91950f9a3a4413f893673a8d5786975cabb7a88d IMAGETOOLS_PROJECT=github.com/opencontainers/image-tools
RUN mkdir -p /go/src/$IMAGETOOLS_PROJECT && \
	git clone https://$IMAGETOOLS_PROJECT /go/src/$IMAGETOOLS_PROJECT && \
	( cd /go/src/$IMAGETOOLS_PROJECT ; git checkout $IMAGETOOLS_VERSION ; ) && \
	make -C /go/src/$IMAGETOOLS_PROJECT tool && \
	install -m0755 /go/src/$IMAGETOOLS_PROJECT/oci-image-tool /usr/bin/oci-image-tool && \
	rm -rf /go/src/$IMAGETOOLS_PROJECT

# Reinstall oci-runtime-tools from source to avoid having to package new versions
# in openSUSE while testing PRs.
# XXX: Switch back to upstream runtime-tools once https://github.com/opencontainers/runtime-tools/pull/432 is merged.
ENV RUNTIMETOOLS_VERSION=c18317bd9b103c5454c2bd2b61cf6d3484e836bf RUNTIMETOOLS_PROJECT=github.com/opencontainers/runtime-tools
RUN mkdir -p /go/src/$RUNTIMETOOLS_PROJECT && \
	git clone https://github.com/cyphar/runtime-tools /go/src/$RUNTIMETOOLS_PROJECT && \
	( cd /go/src/$RUNTIMETOOLS_PROJECT ; git checkout $RUNTIMETOOLS_VERSION ; ) && \
	make -C /go/src/$RUNTIMETOOLS_PROJECT tool && \
	install -m0755 /go/src/$RUNTIMETOOLS_PROJECT/oci-runtime-tool /usr/bin/oci-runtime-tool && \
	rm -rf /go/src/$RUNTIMETOOLS_PROJECT

ENV SOURCE_IMAGE=/opensuse SOURCE_TAG=latest
ARG DOCKER_IMAGE=opensuse/amd64:tumbleweed
RUN skopeo copy docker://$DOCKER_IMAGE oci:$SOURCE_IMAGE:$SOURCE_TAG

VOLUME ["/go/src/github.com/openSUSE/umoci"]
WORKDIR /go/src/github.com/openSUSE/umoci
COPY . /go/src/github.com/openSUSE/umoci
