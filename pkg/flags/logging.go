/*
 * Copyright 2023 The Kubernetes Authors.
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

package flags

import (
	"strings"

	"github.com/spf13/pflag"
	"github.com/urfave/cli/v2"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	logsapi "k8s.io/component-base/logs/api/v1"

	_ "k8s.io/component-base/logs/json/register" // for JSON log output support

	"sigs.k8s.io/dra-example-driver/pkg/featuregates"
)

type LoggingConfig struct {
	config *logsapi.LoggingConfiguration
}

func NewLoggingConfig() *LoggingConfig {
	fg := featuregates.FeatureGates()
	utilruntime.Must(logsapi.AddFeatureGates(fg))
	utilruntime.Must(fg.SetFromMap(map[string]bool{string(logsapi.ContextualLogging): true}))
	return &LoggingConfig{
		config: logsapi.NewLoggingConfiguration(),
	}
}

func (l *LoggingConfig) Apply() error {
	return logsapi.ValidateAndApply(l.config, featuregates.FeatureGates())
}

func (l *LoggingConfig) Flags() []cli.Flag {
	var fs pflag.FlagSet
	logsapi.AddFlags(l.config, &fs)

	var flags []cli.Flag
	fs.VisitAll(func(flag *pflag.Flag) {
		flags = append(flags, pflagToCLI(flag, "Logging:"))
	})
	return flags
}

func pflagToCLI(flag *pflag.Flag, category string) cli.Flag {
	return &cli.GenericFlag{
		Name:        flag.Name,
		Category:    category,
		Usage:       flag.Usage,
		Value:       flag.Value,
		Destination: flag.Value,
		EnvVars:     []string{strings.ToUpper(strings.ReplaceAll(flag.Name, "-", "_"))},
	}
}
