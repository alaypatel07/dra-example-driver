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

	"k8s.io/apimachinery/pkg/types"
)

// metadataWriter handles writing and deleting device metadata JSON files.
type metadataWriter struct {
	baseDir string
}

// newMetadataWriter creates a new metadataWriter.
func newMetadataWriter(baseDir string) (*metadataWriter, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create metadata directory: %w", err)
	}
	return &metadataWriter{baseDir: baseDir}, nil
}

// write writes the device metadata to a JSON file.
func (w *metadataWriter) write(namespace, name string, uid types.UID, metadata *DeviceMetadata) error {
	path := w.getPath(namespace, name, uid)

	// Ensure the parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create metadata subdirectory: %w", err)
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
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

// delete removes the metadata file for a claim and cleans up empty directories.
func (w *metadataWriter) delete(namespace, name string, uid types.UID) error {
	path := w.getPath(namespace, name, uid)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove metadata file: %w", err)
	}

	// Try to remove the claim directory (will fail if not empty, which is fine)
	claimDir := filepath.Dir(path)
	os.Remove(claimDir)

	// Try to remove the namespace directory (will fail if not empty, which is fine)
	nsDir := filepath.Dir(claimDir)
	os.Remove(nsDir)

	return nil
}

// getPath returns the path where the metadata file is written.
// Path format: <baseDir>/<namespace>/<claim-name>/metadata.json
func (w *metadataWriter) getPath(namespace, name string, uid types.UID) string {
	return filepath.Join(w.baseDir, namespace, name, "metadata.json")
}
