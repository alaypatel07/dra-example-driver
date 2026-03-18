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

package featuregates

import (
	"sync"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	DeviceMetadata featuregate.Feature = "DeviceMetadata"
)

var defaultFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	DeviceMetadata: {Default: false, PreRelease: featuregate.Alpha},
}

var (
	featureGatesOnce sync.Once
	featureGates     featuregate.MutableVersionedFeatureGate
)

func FeatureGates() featuregate.MutableVersionedFeatureGate {
	if featureGates == nil {
		featureGatesOnce.Do(func() {
			fg := featuregate.NewFeatureGate()
			utilruntime.Must(fg.Add(defaultFeatureGates))
			featureGates = fg
		})
	}
	return featureGates
}

func Enabled(feature featuregate.Feature) bool {
	return FeatureGates().Enabled(feature)
}

func KnownFeatures() []string {
	return FeatureGates().KnownFeatures()
}
