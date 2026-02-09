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

package metadata

import (
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DeviceMetadata contains metadata about devices allocated to a ResourceClaim.
// This is the internal (unversioned) representation used in-memory.
// It is serialized to versioned JSON files that can be mounted into containers.
type DeviceMetadata struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// Requests contains the device allocation information for each request
	// in the ResourceClaim.
	Requests []DeviceMetadataRequest
}

// DeviceMetadataRequest contains metadata for a single request within a ResourceClaim.
type DeviceMetadataRequest struct {
	// Name is the name of the request (from the ResourceClaim spec).
	Name string

	// Devices contains metadata for each device allocated to this request.
	Devices []Device
}

// Device contains metadata about a single allocated device.
// Fields are aligned with KEP-5304.
type Device struct {
	// Name is the name of the device within the pool.
	Name string

	// Driver is the name of the DRA driver that manages this device.
	Driver string

	// Pool is the name of the resource pool this device belongs to.
	Pool string

	// Attributes contains the device attributes from the ResourceSlice.
	// Uses the Kubernetes DeviceAttribute type for consistency with the
	// resource.k8s.io API.
	Attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute

	// NetworkData contains network-specific device data (e.g., interface name,
	// addresses, hardware address). This is populated for network devices,
	// typically via NRI hooks after CNI runs.
	NetworkData *resourceapi.NetworkDeviceData
}
