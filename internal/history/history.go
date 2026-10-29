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

// Package history keeps a per node history of ended NodeMaintenances, since a NodeMaintenance CR is deleted
// in order to end the maintenance. The history is shown by the console plugin's Node "Maintenances" tab.
package history

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"

	"github.com/medik8s/node-maintenance-operator/v5/api/v1beta1"
)

const (
	// ConfigMapNamePrefix is the prefix of the per node history ConfigMap in the operator namespace,
	// it must match the name used in console-plugin/src/maintenancesTab.tsx
	ConfigMapNamePrefix = "node-maintenance-history-"
	// DataKey is the ConfigMap data key holding the JSON encoded list of records, newest first
	DataKey = "history"
	// MaxRecordsPerNode is the number of records kept per node
	MaxRecordsPerNode = 20

	// LabelHistory marks the history ConfigMaps
	LabelHistory = "nodemaintenance.medik8s.io/history"
	// AnnotationNode holds the node name of a history ConfigMap
	AnnotationNode = "nodemaintenance.medik8s.io/node"
)

// Record describes an ended NodeMaintenance
type Record struct {
	// Name of the NodeMaintenance
	Name string `json:"name"`
	// UID of the NodeMaintenance
	UID string `json:"uid"`
	// Reason of the maintenance
	Reason string `json:"reason,omitempty"`
	// CreatedBy is the user who created the NodeMaintenance, see v1beta1.CreatedByAnnotation
	CreatedBy string `json:"createdBy,omitempty"`
	// StartTime is the creation time of the NodeMaintenance
	StartTime metav1.Time `json:"startTime"`
	// EndTime is the time the maintenance ended, i.e. the node was uncordoned
	EndTime metav1.Time `json:"endTime"`
	// Phase is the last phase of the NodeMaintenance before it ended
	Phase v1beta1.MaintenancePhase `json:"phase,omitempty"`
	// DrainProgress is the last drain progress of the NodeMaintenance before it ended
	DrainProgress int `json:"drainProgress"`
	// LastError is the last error of the NodeMaintenance before it ended
	LastError string `json:"lastError,omitempty"`
}

// ConfigMapName returns the name of the history ConfigMap of the given node,
// and whether the name is valid, which is not the case for very long node names
func ConfigMapName(nodeName string) (string, bool) {
	name := ConfigMapNamePrefix + nodeName
	return name, len(validation.IsDNS1123Subdomain(name)) == 0
}

// RecordEnded adds a record of the ended NodeMaintenance to the history ConfigMap of its node.
// The ConfigMap is owned by the node, so the history is removed together with the node.
func RecordEnded(ctx context.Context, cs kubernetes.Interface, namespace string, node *corev1.Node, nm *v1beta1.NodeMaintenance, endTime time.Time) error {
	name, ok := ConfigMapName(node.Name)
	if !ok {
		return fmt.Errorf("node name %q is too long for a history ConfigMap name", node.Name)
	}
	record := Record{
		Name:          nm.Name,
		UID:           string(nm.UID),
		Reason:        nm.Spec.Reason,
		CreatedBy:     nm.Annotations[v1beta1.CreatedByAnnotation],
		StartTime:     nm.CreationTimestamp,
		EndTime:       metav1.NewTime(endTime),
		Phase:         nm.Status.Phase,
		DrainProgress: nm.Status.DrainProgress,
		LastError:     nm.Status.LastError,
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm, err := cs.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			cm = newConfigMap(name, namespace, node)
			if err := setRecords(cm, addRecord(nil, record)); err != nil {
				return err
			}
			_, err = cs.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
			return err
		} else if err != nil {
			return err
		}

		records, err := GetRecords(cm)
		if err != nil {
			// don't lose new records because of a corrupted history
			records = nil
		}
		if err := setRecords(cm, addRecord(records, record)); err != nil {
			return err
		}
		_, err = cs.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
}

// GetRecords returns the records of the given history ConfigMap, newest first
func GetRecords(cm *corev1.ConfigMap) ([]Record, error) {
	var records []Record
	data, ok := cm.Data[DataKey]
	if !ok || data == "" {
		return records, nil
	}
	if err := json.Unmarshal([]byte(data), &records); err != nil {
		return nil, fmt.Errorf("failed to parse maintenance history of ConfigMap %s: %w", cm.Name, err)
	}
	return records, nil
}

// addRecord prepends the given record, replaces an existing record of the same NodeMaintenance
// (e.g. when the reconcile is retried), and keeps at most MaxRecordsPerNode records
func addRecord(records []Record, record Record) []Record {
	result := []Record{record}
	for _, r := range records {
		if r.UID != record.UID {
			result = append(result, r)
		}
	}
	if len(result) > MaxRecordsPerNode {
		result = result[:MaxRecordsPerNode]
	}
	return result
}

func setRecords(cm *corev1.ConfigMap, records []Record) error {
	data, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("failed to encode maintenance history: %w", err)
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[DataKey] = string(data)
	return nil
}

func newConfigMap(name, namespace string, node *corev1.Node) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      map[string]string{LabelHistory: "true"},
			Annotations: map[string]string{AnnotationNode: node.Name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "v1",
				Kind:               "Node",
				Name:               node.Name,
				UID:                node.UID,
				BlockOwnerDeletion: ptr.To(false),
				Controller:         ptr.To(false),
			}},
		},
	}
}
