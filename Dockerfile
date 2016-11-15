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

FROM opensuse/amd64:42.2
MAINTAINER "Aleksa Sarai <asarai@suse.com>"

# Use my personal repo because currently Go is broken in openSUSE (will be
# fixed in https://build.opensuse.org/request/show/439834) and also several
# things (such as bats and go-mtree) aren't in an proper openSUSE repo.
RUN zypper ar -f -p 10 -g obs://Virtualization:containers obs-vc && \
    zypper ar -f -p 10 -g obs://home:cyphar obs-cyphar && \
	zypper --gpg-auto-import-keys -n ref && \
	zypper -n up
RUN zypper -n in 'go>=1.6' git make skopeo go-mtree bats jq

ENV GOPATH /go
ENV PATH $GOPATH/bin:$PATH
RUN go get -u github.com/golang/lint/golint

ENV SOURCE_IMAGE=/opensuse SOURCE_TAG=latest
ARG DOCKER_IMAGE=opensuse/amd64:tumbleweed
RUN skopeo copy docker://$DOCKER_IMAGE oci:$SOURCE_IMAGE:$SOURCE_TAG

VOLUME ["/go/src/github.com/cyphar/umoci"]
WORKDIR /go/src/github.com/cyphar/umoci
COPY . /go/src/github.com/cyphar/umoci
