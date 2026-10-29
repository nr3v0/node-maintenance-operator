/*
Copyright 2021.

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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/medik8s/node-maintenance-operator/v5/api/v1beta1"
)

const (
	NodeInMaintenanceMetricName = "nmo_node_in_maintenance"

	labelNode            = "node"
	labelNodeMaintenance = "nodemaintenance"
	labelReason          = "reason"
)

// NodeInMaintenance is 1 for every node that is under an active (not failed) NodeMaintenance.
// The NodeInMaintenance alert is based on this metric.
var NodeInMaintenance = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: NodeInMaintenanceMetricName,
		Help: "Indicates whether a node is in maintenance (1) because of a NodeMaintenance CR, along with the maintenance reason",
	},
	[]string{labelNode, labelNodeMaintenance, labelReason},
)

func init() {
	metrics.Registry.MustRegister(NodeInMaintenance)
}

// SetNodeInMaintenance marks the node of the given NodeMaintenance as being in maintenance.
// Series of a previous reason of the same CR are removed first.
func SetNodeInMaintenance(nm *v1beta1.NodeMaintenance) {
	ClearNodeInMaintenance(nm.Name)
	NodeInMaintenance.With(prometheus.Labels{
		labelNode:            nm.Spec.NodeName,
		labelNodeMaintenance: nm.Name,
		labelReason:          nm.Spec.Reason,
	}).Set(1)
}

// ClearNodeInMaintenance removes all series of the NodeMaintenance with the given name.
func ClearNodeInMaintenance(nmName string) {
	NodeInMaintenance.DeletePartialMatch(prometheus.Labels{labelNodeMaintenance: nmName})
}
