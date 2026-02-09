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

package downwardapihelper

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"sigs.k8s.io/dra-example-driver/pkg/metadata/v1alpha1"
)

// metadataWriter handles writing and deleting device metadata JSON files.
// Files follow the KEP-5304 path convention:
//
//	<baseDir>/<namespace>_<claimName>/<requestName>/<driverName>-metadata.json
type metadataWriter struct {
	baseDir    string
	driverName string
}

// newMetadataWriter creates a new metadataWriter.
func newMetadataWriter(baseDir, driverName string) (*metadataWriter, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create metadata directory: %w", err)
	}
	return &metadataWriter{baseDir: baseDir, driverName: driverName}, nil
}

// write writes the device metadata for a single request to a JSON file.
func (w *metadataWriter) write(namespace, claimName, requestName string, dm *v1alpha1.DeviceMetadata) error {
	path := w.getPath(namespace, claimName, requestName)

	// Ensure the parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create metadata subdirectory: %w", err)
	}

	data, err := json.MarshalIndent(dm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal device metadata: %w", err)
	}

	// Write atomically using a temp file + rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write metadata temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename metadata file: %w", err)
	}

	return nil
}

// deleteClaimDir removes the entire claim directory and all metadata files within it.
func (w *metadataWriter) deleteClaimDir(namespace, claimName string) error {
	claimDir := w.getClaimDir(namespace, claimName)
	if err := os.RemoveAll(claimDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove claim metadata directory: %w", err)
	}
	return nil
}

// getPath returns the path for a per-request metadata file.
// Path format: <baseDir>/<namespace>_<claimName>/<requestName>/<driverName>-metadata.json
func (w *metadataWriter) getPath(namespace, claimName, requestName string) string {
	return filepath.Join(w.getClaimDir(namespace, claimName), requestName, w.driverName+"-metadata.json")
}

// getClaimDir returns the claim directory path.
// Path format: <baseDir>/<namespace>_<claimName>
func (w *metadataWriter) getClaimDir(namespace, claimName string) string {
	return filepath.Join(w.baseDir, namespace+"_"+claimName)
}

// getContainerPath returns the container-side path for a per-request metadata file.
// Path format: /var/run/dra-device-attributes/<claimName>/<requestName>/<driverName>-metadata.json
func (w *metadataWriter) getContainerPath(claimName, requestName string) string {
	return filepath.Join("/var/run/dra-device-attributes", claimName, requestName, w.driverName+"-metadata.json")
}
