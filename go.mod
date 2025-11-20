// SPDX-License-Identifier: Apache-2.0
// umoci: Umoci Modifies Open Containers' Images
// Copyright (C) 2016-2025 SUSE LLC
// Copyright (C) 2018, 2020 Cisco Systems
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

module github.com/opencontainers/umoci

go 1.24.0

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6
	github.com/apex/log v1.9.0
	github.com/blang/semver/v4 v4.0.0
	github.com/containerd/platforms v0.2.1
	github.com/cyphar/filepath-securejoin v0.6.1
	github.com/docker/go-units v0.5.0
	github.com/klauspost/compress v1.11.3
	github.com/klauspost/pgzip v1.2.6
	github.com/moby/sys/user v0.4.0
	github.com/moby/sys/userns v0.1.0
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.1
	github.com/opencontainers/runtime-spec v1.3.0
	github.com/rootless-containers/proto/go-proto v0.0.0-20230421021042-4cd87ebadd67
	github.com/stretchr/testify v1.11.1
	github.com/urfave/cli v1.22.12
	github.com/vbatts/go-mtree v0.6.1-0.20250911112631-8307d76bc1b9
	golang.org/x/sys v0.38.0
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/containerd/log v0.1.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// NOTE: This breaks "go install" and so we really need to remove this
// directive as soon as possible. This is needed because we are a waiting for
// upstream to merge the following PRs:
// * https://github.com/vbatts/go-mtree/pull/211
// * https://github.com/vbatts/go-mtree/pull/212
// * https://github.com/vbatts/go-mtree/pull/214
replace github.com/vbatts/go-mtree => github.com/cyphar/go-mtree v0.0.0-20250928235313-918fa724e2fe
