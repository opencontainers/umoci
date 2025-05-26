#!/bin/bash
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

set -Eeuo pipefail

go mod tidy && go mod vendor
go get github.com/AdaLogics/go-fuzz-headers@latest
go mod vendor

compile_go_fuzzer github.com/opencontainers/umoci/oci/casext Fuzz casext_fuzz
compile_go_fuzzer github.com/opencontainers/umoci/oci/layer FuzzUnpack fuzz_unpack
compile_go_fuzzer github.com/opencontainers/umoci/oci/layer FuzzGenerateLayer fuzz_generate_layer
compile_go_fuzzer github.com/opencontainers/umoci/mutate FuzzMutate fuzz_mutate
compile_go_fuzzer github.com/opencontainers/umoci/pkg/hardening Fuzz fuzz_hardening
