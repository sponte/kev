/**
 * Copyright 2020 Appvia Ltd <info@appvia.io>
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

package kev

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/appvia/kev/pkg/kev/config"

	composego "github.com/compose-spec/compose-go/types"
	"k8s.io/apimachinery/pkg/api/resource"
)

// setDefaultLabels sets sensible workload defaults as labels.
func setDefaultLabels(target *ServiceConfig) {
	target.Labels.Add(config.LabelWorkloadImagePullPolicy, config.DefaultImagePullPolicy)
	target.Labels.Add(config.LabelWorkloadServiceAccountName, config.DefaultServiceAccountName)
}

// TODO: Remove this whole thing when ready
// extractVolumesLabels extracts volume labels into a label's Volumes attribute.
func extractVolumesLabels(c *ComposeProject, out *composeOverride) {
	vols := make(map[string]VolumeConfig)

	for _, v := range c.VolumeNames() {
		labels := map[string]string{}

		if storageClass, ok := c.Volumes[v].Labels[config.LabelVolumeStorageClass]; ok {
			labels[config.LabelVolumeStorageClass] = storageClass
		} else {
			labels[config.LabelVolumeStorageClass] = config.DefaultVolumeStorageClass
		}

		if volSize, ok := c.Volumes[v].Labels[config.LabelVolumeSize]; ok {
			labels[config.LabelVolumeSize] = volSize
		} else {
			labels[config.LabelVolumeSize] = config.DefaultVolumeSize
		}

		vols[v] = VolumeConfig{Labels: labels}
	}
	out.Volumes = vols
}

// extractDeploymentLabels extracts deployment related into a label's Service.
func extractDeploymentLabels(source composego.ServiceConfig, target *ServiceConfig) {
	// extractWorkloadType(source, target)
	// extractWorkloadReplicas(source, target)
	// extractWorkloadRestartPolicy(source, target)
	extractWorkloadResourceRequests(source, target)
	extractWorkloadResourceLimits(source, target)
	extractWorkloadRollingUpdatePolicy(source, target)
}

// extractWorkloadRollingUpdatePolicy extracts deployment's rolling update policy.
func extractWorkloadRollingUpdatePolicy(source composego.ServiceConfig, target *ServiceConfig) {
	if source.Deploy != nil && source.Deploy.UpdateConfig != nil {
		value := strconv.FormatUint(*source.Deploy.UpdateConfig.Parallelism, 10)
		target.Labels.Add(config.LabelWorkloadRollingUpdateMaxSurge, value)
	}
}

// extractWorkloadResourceLimits extracts deployment's resource limits.
func extractWorkloadResourceLimits(source composego.ServiceConfig, target *ServiceConfig) {
	if source.Deploy != nil && source.Deploy.Resources.Limits != nil {
		target.Labels.Add(config.LabelWorkloadMaxCPU, source.Deploy.Resources.Limits.NanoCPUs)

		value := getMemoryQuantity(int64(source.Deploy.Resources.Limits.MemoryBytes))
		target.Labels.Add(config.LabelWorkloadMaxMemory, value)
	}
}

// extractWorkloadResourceRequests extracts deployment's resource requests.
func extractWorkloadResourceRequests(source composego.ServiceConfig, target *ServiceConfig) {
	if source.Deploy != nil && source.Deploy.Resources.Reservations != nil {
		target.Labels.Add(config.LabelWorkloadCPU, source.Deploy.Resources.Reservations.NanoCPUs)

		value := getMemoryQuantity(int64(source.Deploy.Resources.Reservations.MemoryBytes))
		target.Labels.Add(config.LabelWorkloadMemory, value)
	}
}

// formatSlice formats a string slice as '["value1", "value2", "value3"]'
func formatSlice(test []string) string {
	quoted := fmt.Sprintf("[%q]", strings.Join(test, `", "`))
	return strings.ReplaceAll(quoted, "\\", "")
}

// GetMemoryQuantity returns memory amount as string in Kubernetes notation
// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory
// Example: 100Mi, 20Gi
func getMemoryQuantity(b int64) string {
	const unit int64 = 1024

	q := resource.NewQuantity(b, resource.BinarySI)

	quantity, _ := q.AsInt64()
	if quantity%unit == 0 {
		return q.String()
	}

	// Kubernetes resource quantity computation doesn't do well with values containing decimal points
	// Example: 10.6Mi would translate to 11114905 (bytes)
	// Let's keep consistent with kubernetes resource amount notation (below).

	if b < unit {
		return fmt.Sprintf("%d", b)
	}

	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f%ci", float64(b)/float64(div), "KMGTPE"[exp])
}
