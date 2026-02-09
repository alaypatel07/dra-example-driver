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
	"context"
	"fmt"
	"sync"

	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/dynamic-resource-allocation/kubeletplugin"
	"k8s.io/dynamic-resource-allocation/resourceslice"
	"k8s.io/klog/v2"
	cdiapi "tags.cncf.io/container-device-interface/pkg/cdi"
	cdiparser "tags.cncf.io/container-device-interface/pkg/parser"
	cdispec "tags.cncf.io/container-device-interface/specs-go"

	"sigs.k8s.io/dra-example-driver/pkg/metadata/v1alpha1"
)

// Helper wraps kubeletplugin.Helper and adds automatic device metadata JSON handling.
type Helper struct {
	kubeletHelper *kubeletplugin.Helper
	driverName    string

	// Metadata handling
	metadataWriter *metadataWriter

	// CDI handling for metadata mount
	cdiCache *cdiapi.Cache

	// Device data from PublishResources, used for attribute lookup
	mutex         sync.RWMutex
	devicesByPool map[string]map[string]resourceapi.Device // pool -> device name -> device
}

// Option configures the Helper.
type Option func(*options) error

type options struct {
	metadataPath string
	cdiRoot      string
}

// DeviceMetadataJSON enables writing device metadata JSON files.
// When enabled, the server automatically:
//   - Writes per-request metadata after PrepareResourceClaims succeeds
//   - Deletes metadata after UnprepareResourceClaims succeeds
//   - Includes device attributes from PublishResources()
//   - Mounts the metadata files into containers via CDI
//
// Parameters:
//   - metadataPath: base dir for metadata JSON files (e.g., /var/run/dra-device-attributes)
//   - cdiRoot: where CDI specs are written (e.g., /var/run/cdi)
func DeviceMetadataJSON(metadataPath, cdiRoot string) Option {
	return func(o *options) error {
		o.metadataPath = metadataPath
		o.cdiRoot = cdiRoot
		return nil
	}
}

// Start creates and starts a new Helper that wraps the kubeletplugin framework.
func Start(
	ctx context.Context,
	plugin kubeletplugin.DRAPlugin,
	driverName string,
	kubeletOpts []kubeletplugin.Option,
	opts ...Option,
) (*Helper, error) {
	var helperOpts options
	for _, opt := range opts {
		if err := opt(&helperOpts); err != nil {
			return nil, err
		}
	}

	h := &Helper{
		driverName:    driverName,
		devicesByPool: make(map[string]map[string]resourceapi.Device),
	}

	if helperOpts.metadataPath != "" {
		writer, err := newMetadataWriter(helperOpts.metadataPath, driverName)
		if err != nil {
			return nil, err
		}
		h.metadataWriter = writer
		klog.V(2).Infof("Device metadata will be written to: %s", helperOpts.metadataPath)

		// Initialize CDI cache for metadata mounts
		if helperOpts.cdiRoot != "" {
			cdiCache, err := cdiapi.NewCache(cdiapi.WithSpecDirs(helperOpts.cdiRoot))
			if err != nil {
				return nil, fmt.Errorf("create CDI cache: %w", err)
			}
			h.cdiCache = cdiCache
			klog.V(2).Infof("CDI specs for metadata mounts will be written to: %s", helperOpts.cdiRoot)
		}
	}

	wrapper := &pluginWrapper{
		inner:  plugin,
		helper: h,
	}

	kubeletHelper, err := kubeletplugin.Start(ctx, wrapper, kubeletOpts...)
	if err != nil {
		return nil, err
	}
	h.kubeletHelper = kubeletHelper

	return h, nil
}

// Stop stops the helper.
func (h *Helper) Stop() {
	h.kubeletHelper.Stop()
}

// PublishResources publishes device resources and stores device data for metadata attribute lookup.
func (h *Helper) PublishResources(ctx context.Context, resources resourceslice.DriverResources) error {
	h.mutex.Lock()
	h.devicesByPool = make(map[string]map[string]resourceapi.Device)
	for poolName, pool := range resources.Pools {
		h.devicesByPool[poolName] = make(map[string]resourceapi.Device)
		for _, slice := range pool.Slices {
			for _, device := range slice.Devices {
				h.devicesByPool[poolName][device.Name] = device
			}
		}
	}
	h.mutex.Unlock()

	return h.kubeletHelper.PublishResources(ctx, resources)
}

// RegistrationStatus returns the registration status from the underlying helper.
func (h *Helper) RegistrationStatus() interface{} {
	return h.kubeletHelper.RegistrationStatus()
}

// getDeviceAttributes looks up attributes for a device.
// Returns an error if the pool or device is not found, which indicates
// an inconsistency between PublishResources and PrepareResourceClaims.
func (h *Helper) getDeviceAttributes(poolName, deviceName string) (map[resourceapi.QualifiedName]resourceapi.DeviceAttribute, error) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	pool, ok := h.devicesByPool[poolName]
	if !ok {
		return nil, fmt.Errorf("pool %q not found in device data - was PublishResources called?", poolName)
	}
	device, ok := pool[deviceName]
	if !ok {
		return nil, fmt.Errorf("device %q not found in pool %q - was PublishResources called with this device?", deviceName, poolName)
	}

	// Return the attributes directly from the device
	return device.Attributes, nil
}

// writeMetadataForClaim builds and writes per-request metadata files for a prepared claim.
// Each request gets its own file at:
//
//	<baseDir>/<namespace>_<claimName>/<requestName>/<driverName>-metadata.json
func (h *Helper) writeMetadataForClaim(claim *resourceapi.ResourceClaim, devices []kubeletplugin.Device) error {
	if h.metadataWriter == nil {
		return nil
	}

	// Build a map of request name -> devices
	requestDevicesMap := make(map[string][]kubeletplugin.Device)
	for _, device := range devices {
		for _, requestName := range device.Requests {
			requestDevicesMap[requestName] = append(requestDevicesMap[requestName], device)
		}
		if len(device.Requests) == 0 {
			requestDevicesMap[""] = append(requestDevicesMap[""], device)
		}
	}

	// Write one metadata file per request
	for _, request := range claim.Spec.Devices.Requests {
		dm, err := h.buildRequestMetadata(claim, request.Name, requestDevicesMap[request.Name])
		if err != nil {
			return fmt.Errorf("build metadata for request %q: %w", request.Name, err)
		}
		if err := h.metadataWriter.write(claim.Namespace, claim.Name, request.Name, dm); err != nil {
			return fmt.Errorf("write metadata for request %q: %w", request.Name, err)
		}
		klog.V(4).Infof("Wrote metadata for claim %s/%s request %s", claim.Namespace, claim.Name, request.Name)
	}

	return nil
}

// deleteMetadataForClaim removes all metadata files for a claim.
func (h *Helper) deleteMetadataForClaim(namespace, name string, uid types.UID) error {
	if h.metadataWriter == nil {
		return nil
	}
	return h.metadataWriter.deleteClaimDir(namespace, name)
}

// cdiVendor returns the CDI vendor name for this driver.
func (h *Helper) cdiVendor() string {
	return "k8s." + h.driverName
}

// cdiClass returns the CDI class for metadata.
const cdiMetadataClass = "metadata"

// metadataCDIDeviceName returns the CDI device name for a request's metadata,
// following the KEP-5304 convention: {claimNamespace}_{claimName}_{requestName}_metadata
func metadataCDIDeviceName(namespace, claimName, requestName string) string {
	return namespace + "_" + claimName + "_" + requestName + "_metadata"
}

// createMetadataCDISpecs creates a CDI spec with one device per request,
// each mounting that request's metadata file into the container.
// Returns a map of requestName -> CDI device ID.
//
// CDI device ID format (per KEP-5304):
//
//	{driverName}/metadata={claimNamespace}_{claimName}_{requestName}_metadata
func (h *Helper) createMetadataCDISpecs(claim *resourceapi.ResourceClaim) (map[string]string, error) {
	if h.cdiCache == nil {
		return nil, nil
	}

	// Build one CDI device per request
	var devices []cdispec.Device
	cdiDeviceIDs := make(map[string]string) // requestName -> qualified CDI device ID

	for _, request := range claim.Spec.Devices.Requests {
		deviceName := metadataCDIDeviceName(claim.Namespace, claim.Name, request.Name)
		hostPath := h.metadataWriter.getPath(claim.Namespace, claim.Name, request.Name)
		containerPath := h.metadataWriter.getContainerPath(claim.Name, request.Name)

		devices = append(devices, cdispec.Device{
			Name: deviceName,
			ContainerEdits: cdispec.ContainerEdits{
				Mounts: []*cdispec.Mount{
					{
						HostPath:      hostPath,
						ContainerPath: containerPath,
						Options:       []string{"ro", "bind"},
					},
				},
			},
		})

		cdiDeviceIDs[request.Name] = cdiparser.QualifiedName(h.cdiVendor(), cdiMetadataClass, deviceName)
	}

	spec := &cdispec.Spec{
		Kind:    h.cdiVendor() + "/" + cdiMetadataClass,
		Devices: devices,
	}

	minVersion, err := cdiapi.MinimumRequiredVersion(spec)
	if err != nil {
		return nil, fmt.Errorf("get minimum CDI version: %w", err)
	}
	spec.Version = minVersion

	specName := cdiapi.GenerateTransientSpecName(h.cdiVendor(), cdiMetadataClass, string(claim.UID))
	if err := h.cdiCache.WriteSpec(spec, specName); err != nil {
		return nil, fmt.Errorf("write CDI spec: %w", err)
	}

	klog.V(4).Infof("Created CDI spec for metadata mounts: claim %s/%s -> %v", claim.Namespace, claim.Name, cdiDeviceIDs)
	return cdiDeviceIDs, nil
}

// deleteMetadataCDISpec removes the CDI spec for a claim's metadata.
func (h *Helper) deleteMetadataCDISpec(uid types.UID) error {
	if h.cdiCache == nil {
		return nil
	}

	specName := cdiapi.GenerateTransientSpecName(h.cdiVendor(), cdiMetadataClass, string(uid))
	if err := h.cdiCache.RemoveSpec(specName); err != nil {
		return fmt.Errorf("remove CDI spec: %w", err)
	}
	klog.V(4).Infof("Deleted CDI spec for metadata: %s", specName)
	return nil
}

// buildRequestMetadata constructs a DeviceMetadata for a single request within a claim.
func (h *Helper) buildRequestMetadata(claim *resourceapi.ResourceClaim, requestName string, devices []kubeletplugin.Device) (*v1alpha1.DeviceMetadata, error) {
	dm := &v1alpha1.DeviceMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "DeviceMetadata",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      claim.Name,
			Namespace: claim.Namespace,
			UID:       claim.UID,
		},
		Requests: []v1alpha1.DeviceMetadataRequest{
			{
				Name:    requestName,
				Devices: []v1alpha1.Device{},
			},
		},
	}

	for _, device := range devices {
		attrs, err := h.getDeviceAttributes(device.PoolName, device.DeviceName)
		if err != nil {
			return nil, fmt.Errorf("get attributes for device %s/%s: %w", device.PoolName, device.DeviceName, err)
		}

		dm.Requests[0].Devices = append(dm.Requests[0].Devices, v1alpha1.Device{
			Name:       device.DeviceName,
			Driver:     h.driverName,
			Pool:       device.PoolName,
			Attributes: attrs,
		})
	}

	return dm, nil
}

// pluginWrapper wraps the user's DRAPlugin to intercept prepare/unprepare calls.
type pluginWrapper struct {
	inner  kubeletplugin.DRAPlugin
	helper *Helper
}

func (w *pluginWrapper) PrepareResourceClaims(ctx context.Context, claims []*resourceapi.ResourceClaim) (map[types.UID]kubeletplugin.PrepareResult, error) {
	result, err := w.inner.PrepareResourceClaims(ctx, claims)
	if err != nil {
		return result, err
	}

	claimByUID := make(map[types.UID]*resourceapi.ResourceClaim)
	for _, claim := range claims {
		claimByUID[claim.UID] = claim
	}

	for uid, prepareResult := range result {
		if prepareResult.Err == nil && w.helper.metadataWriter != nil {
			claim := claimByUID[uid]

			// Write per-request metadata JSON files
			if err := w.helper.writeMetadataForClaim(claim, prepareResult.Devices); err != nil {
				klog.Errorf("Failed to write device metadata for claim %v, unpreparing and failing allocation: %v", uid, err)
				w.unprepareOnFailure(ctx, uid, claim)
				result[uid] = kubeletplugin.PrepareResult{
					Err: fmt.Errorf("write device metadata: %w", err),
				}
				continue
			}

			// Create per-request CDI specs for metadata mounts
			cdiDeviceIDs, err := w.helper.createMetadataCDISpecs(claim)
			if err != nil {
				klog.Errorf("Failed to create CDI spec for claim %v, unpreparing and failing allocation: %v", uid, err)
				w.helper.deleteMetadataForClaim(claim.Namespace, claim.Name, claim.UID)
				w.unprepareOnFailure(ctx, uid, claim)
				result[uid] = kubeletplugin.PrepareResult{
					Err: fmt.Errorf("create metadata CDI spec: %w", err),
				}
				continue
			}

			// Add per-request metadata CDI device IDs to each device's CDI list.
			// Each device gets the metadata CDI IDs for the requests it serves.
			if len(cdiDeviceIDs) > 0 {
				updatedDevices := make([]kubeletplugin.Device, len(prepareResult.Devices))
				for i, device := range prepareResult.Devices {
					updatedDevices[i] = device
					updatedDevices[i].CDIDeviceIDs = append([]string{}, device.CDIDeviceIDs...)
					for _, requestName := range device.Requests {
						if metadataCDIID, ok := cdiDeviceIDs[requestName]; ok {
							updatedDevices[i].CDIDeviceIDs = append(updatedDevices[i].CDIDeviceIDs, metadataCDIID)
						}
					}
				}
				result[uid] = kubeletplugin.PrepareResult{Devices: updatedDevices}
			}
		}
	}

	return result, nil
}

// unprepareOnFailure cleans up a claim after metadata/CDI failure.
func (w *pluginWrapper) unprepareOnFailure(ctx context.Context, uid types.UID, claim *resourceapi.ResourceClaim) {
	unprepareResult, unprepareErr := w.inner.UnprepareResourceClaims(ctx, []kubeletplugin.NamespacedObject{
		{UID: uid, NamespacedName: types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name}},
	})
	if unprepareErr != nil {
		klog.Errorf("Failed to unprepare claim %v after metadata failure: %v", uid, unprepareErr)
	} else if claimErr := unprepareResult[uid]; claimErr != nil {
		klog.Errorf("Failed to unprepare claim %v after metadata failure: %v", uid, claimErr)
	}
}

func (w *pluginWrapper) UnprepareResourceClaims(ctx context.Context, claims []kubeletplugin.NamespacedObject) (map[types.UID]error, error) {
	result, err := w.inner.UnprepareResourceClaims(ctx, claims)

	for _, claim := range claims {
		// Delete CDI spec for metadata mount
		if deleteErr := w.helper.deleteMetadataCDISpec(claim.UID); deleteErr != nil {
			klog.Warningf("Failed to delete CDI spec for claim %v: %v", claim.UID, deleteErr)
		}
		// Delete metadata directory for this claim
		if deleteErr := w.helper.deleteMetadataForClaim(claim.Namespace, claim.Name, claim.UID); deleteErr != nil {
			klog.Warningf("Failed to delete device metadata for claim %v: %v", claim.UID, deleteErr)
		}
	}

	return result, err
}

func (w *pluginWrapper) HandleError(ctx context.Context, err error, msg string) {
	if handler, ok := w.inner.(interface {
		HandleError(context.Context, error, string)
	}); ok {
		handler.HandleError(ctx, err, msg)
	}
}
