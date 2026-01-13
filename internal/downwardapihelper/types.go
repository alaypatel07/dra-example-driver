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

// Package downwardapihelper provides a wrapper around kubeletplugin that automatically
// handles device metadata JSON files, similar to Kubernetes downward API.
package downwardapihelper

import (
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// APIVersion is the API version for DeviceMetadata.
	APIVersion = "resource.k8s.io/v1alpha1"
	// Kind is the Kind for DeviceMetadata.
	Kind = "DeviceMetadata"

	// PodClaimNameAnnotation is the annotation key set by the kubelet on
	// ResourceClaims created from a template. It contains the pod-local
	// name used to reference the claim in the pod spec.
	PodClaimNameAnnotation = "resource.kubernetes.io/pod-claim-name"
)

// DeviceMetadata contains metadata about devices allocated to a ResourceClaim.
// This is written as a JSON file that can be mounted into containers.
type DeviceMetadata struct {
	metav1.TypeMeta `json:",inline"`
	// ObjectMeta contains claim identification (name, namespace, UID).
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// PodClaimName is the name used by the pod to reference this claim in its spec.
	// This is only set for claims created from a template (resourceClaimTemplateName).
	// For pre-existing claims referenced via resourceClaimName, this will be empty.
	// Populated from the "resource.kubernetes.io/pod-claim-name" annotation.
	PodClaimName string `json:"podClaimName,omitempty"`

	// Requests contains the device allocation information for each request
	// in the ResourceClaim.
	Requests []RequestMetadata `json:"requests,omitempty"`
}

// RequestMetadata contains metadata for a single request within a ResourceClaim.
type RequestMetadata struct {
	// Name is the name of the request (from the ResourceClaim spec).
	Name string `json:"name"`

	// Devices contains metadata for each device allocated to this request.
	Devices []DeviceInfo `json:"devices,omitempty"`
}

// DeviceInfo contains metadata about a single allocated device.
type DeviceInfo struct {
	// Device is the name of the device within the pool.
	Device string `json:"device"`

	// Attributes contains the device attributes from the ResourceSlice.
	// These are automatically populated from the device data passed to PublishResources.
	// Uses the Kubernetes DeviceAttribute type for consistency with the API.
	Attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute `json:"attributes,omitempty"`

	// Data contains driver-provided runtime-discovered data.
	// This is populated via the DriverDataProvider callback if configured.
	// Drivers can provide arbitrary structured data here.
	Data runtime.RawExtension `json:"data,omitempty"`
}

// DeviceAttribute represents a single device attribute.
// Only one of the value fields should be set.
type DeviceAttribute struct {
	// IntValue is set for integer attributes.
	IntValue *int64 `json:"intValue,omitempty"`
	// BoolValue is set for boolean attributes.
	BoolValue *bool `json:"boolValue,omitempty"`
	// StringValue is set for string attributes.
	StringValue *string `json:"stringValue,omitempty"`
	// VersionValue is set for version attributes (semantic version string).
	VersionValue *string `json:"versionValue,omitempty"`
}
