#!/usr/bin/env bash

# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script runs the Kubernetes code generators for the metadata API types.
# Prerequisites: deepcopy-gen, conversion-gen, defaulter-gen must be installed.
#   go install k8s.io/code-generator/cmd/deepcopy-gen@latest
#   go install k8s.io/code-generator/cmd/conversion-gen@latest
#   go install k8s.io/code-generator/cmd/defaulter-gen@latest

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BOILERPLATE="${SCRIPT_ROOT}/hack/boilerplate.go.txt"

MODULE="sigs.k8s.io/dra-example-driver"
INTERNAL_PKG="${MODULE}/pkg/metadata"
V1ALPHA1_PKG="${MODULE}/pkg/metadata/v1alpha1"

echo "Generating deepcopy functions..."
deepcopy-gen \
  --output-file zz_generated.deepcopy.go \
  --go-header-file "${BOILERPLATE}" \
  "${INTERNAL_PKG}" \
  "${V1ALPHA1_PKG}"

echo "Generating conversion functions..."
conversion-gen \
  --output-file zz_generated.conversion.go \
  --go-header-file "${BOILERPLATE}" \
  "${V1ALPHA1_PKG}"

echo "Generating defaulter functions..."
defaulter-gen \
  --output-file zz_generated.defaults.go \
  --go-header-file "${BOILERPLATE}" \
  "${V1ALPHA1_PKG}"

echo "Code generation complete."
