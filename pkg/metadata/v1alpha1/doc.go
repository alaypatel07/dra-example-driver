/*
Copyright 2025 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// +k8s:deepcopy-gen=package
// +k8s:conversion-gen=sigs.k8s.io/dra-example-driver/pkg/metadata
// +k8s:defaulter-gen=TypeMeta
// +groupName=metadata.resource.k8s.io

// Package v1alpha1 contains the v1alpha1 version of the DRA device metadata
// API types. These are the external (versioned) types that get serialized
// to JSON metadata files.
//
// When the schema evolves, new version packages (e.g., v1beta1) are added
// alongside this one, with conversion functions to translate between versions.
package v1alpha1
