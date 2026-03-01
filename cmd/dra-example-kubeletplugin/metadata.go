/*
 * Copyright The Kubernetes Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	resourceapi "k8s.io/api/resource/v1"
	"k8s.io/klog/v2"
	cdispec "tags.cncf.io/container-device-interface/specs-go"

	"sigs.k8s.io/dra-example-driver/internal/profiles"
)

const (
	metadataSubDir            = "dra-device-metadata"
	containerMetadataBasePath = "/var/run/dra-device-attributes"
)

// KEP-5304 metadata JSON types — matches what kubevirt's virt-launcher expects.

type deviceMetadata struct {
	APIVersion   string                  `json:"apiVersion"`
	Kind         string                  `json:"kind"`
	Metadata     deviceMetadataObjMeta   `json:"metadata"`
	PodClaimName *string                 `json:"podClaimName,omitempty"`
	Requests     []deviceMetadataRequest `json:"requests,omitempty"`
}

type deviceMetadataObjMeta struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	UID        string `json:"uid"`
	Generation int64  `json:"generation"`
}

type deviceMetadataRequest struct {
	Name    string           `json:"name"`
	Devices []metadataDevice `json:"devices,omitempty"`
}

type metadataDevice struct {
	Name       string                                                     `json:"name"`
	Driver     string                                                     `json:"driver"`
	Pool       string                                                     `json:"pool"`
	Attributes map[resourceapi.QualifiedName]resourceapi.DeviceAttribute  `json:"attributes,omitempty"`
}

// writeClaimMetadata writes KEP-5304 metadata JSON files for a claim and returns
// CDI mount specs that bind-mount them into the container at the well-known path.
func writeClaimMetadata(
	pluginDir string,
	driverName string,
	claim *resourceapi.ResourceClaim,
	preparedDevices profiles.PreparedDevices,
	allocatable AllocatableDevices,
) ([]*cdispec.Mount, error) {
	claimUID := string(claim.UID)
	claimName := claim.Name
	claimNs := claim.Namespace

	requestDevices := make(map[string][]metadataDevice)
	for _, pd := range preparedDevices {
		device, ok := allocatable[pd.GetDeviceName()]
		if !ok {
			continue
		}
		md := metadataDevice{
			Name:       pd.GetDeviceName(),
			Driver:     driverName,
			Pool:       pd.GetPoolName(),
			Attributes: device.Attributes,
		}
		for _, reqName := range pd.GetRequestNames() {
			requestDevices[reqName] = append(requestDevices[reqName], md)
		}
	}

	var mounts []*cdispec.Mount
	for reqName, devices := range requestDevices {
		metadata := deviceMetadata{
			APIVersion:   "metadata.resource.k8s.io/v1alpha1",
			Kind:         "DeviceMetadata",
			Metadata: deviceMetadataObjMeta{
				Name:       claimName,
				Namespace:  claimNs,
				UID:        claimUID,
				Generation: 1,
			},
			PodClaimName: podClaimName(claim),
			Requests: []deviceMetadataRequest{
				{
					Name:    reqName,
					Devices: devices,
				},
			},
		}

		hostDir := filepath.Join(pluginDir, metadataSubDir, claimUID, reqName)
		if err := os.MkdirAll(hostDir, 0755); err != nil {
			return nil, fmt.Errorf("create metadata dir: %w", err)
		}

		hostPath := filepath.Join(hostDir, "metadata.json")
		data, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal metadata: %w", err)
		}
		if err := os.WriteFile(hostPath, data, 0644); err != nil {
			return nil, fmt.Errorf("write metadata file: %w", err)
		}

		containerPath := filepath.Join(
			containerMetadataBasePath, claimName, reqName, driverName+"-metadata.json",
		)
		mounts = append(mounts, &cdispec.Mount{
			HostPath:      hostPath,
			ContainerPath: containerPath,
			Options:       []string{"ro", "bind"},
		})

		klog.Infof("Wrote metadata for claim %s/%s request %s -> %s", claimNs, claimName, reqName, hostPath)
	}

	return mounts, nil
}

const podResourceClaimAnnotation = "resource.kubernetes.io/pod-claim-name"

// podClaimName extracts the pod-level claim reference name from the
// annotation set by the ResourceClaim controller on template-generated claims.
// Returns nil for pre-existing claims (created directly, not from a template).
func podClaimName(claim *resourceapi.ResourceClaim) *string {
	if v, ok := claim.Annotations[podResourceClaimAnnotation]; ok {
		return &v
	}
	return nil
}

// deleteClaimMetadata removes the metadata directory for a claim.
func deleteClaimMetadata(pluginDir string, claimUID string) error {
	dir := filepath.Join(pluginDir, metadataSubDir, claimUID)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove metadata dir: %w", err)
	}
	return nil
}
