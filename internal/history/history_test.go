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

package history

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/medik8s/node-maintenance-operator/v5/api/v1beta1"
)

const (
	testNamespace = "test-namespace"
	testNodeName  = "node01"
)

var _ = Describe("Maintenance history", func() {
	var (
		cs   *fake.Clientset
		ctx  = context.Background()
		node *corev1.Node
	)

	BeforeEach(func() {
		cs = fake.NewSimpleClientset()
		node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName, UID: "node-uid"}}
	})

	getRecords := func() []Record {
		name, _ := ConfigMapName(testNodeName)
		cm, err := cs.CoreV1().ConfigMaps(testNamespace).Get(ctx, name, metav1.GetOptions{})
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		records, err := GetRecords(cm)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		return records
	}

	When("the first maintenance of a node ends", func() {
		It("should create the history ConfigMap owned by the node", func() {
			nm := getTestNM("nm-1", "uid-1")
			nm.Status.Phase = v1beta1.MaintenanceSucceeded
			nm.Status.DrainProgress = 100
			end := time.Now()
			Expect(RecordEnded(ctx, cs, testNamespace, node, nm, end)).To(Succeed())

			cm, err := cs.CoreV1().ConfigMaps(testNamespace).Get(ctx, ConfigMapNamePrefix+testNodeName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(cm.Labels).To(HaveKeyWithValue(LabelHistory, "true"))
			Expect(cm.Annotations).To(HaveKeyWithValue(AnnotationNode, testNodeName))
			Expect(cm.OwnerReferences).To(HaveLen(1))
			Expect(cm.OwnerReferences[0].Kind).To(Equal("Node"))
			Expect(cm.OwnerReferences[0].UID).To(Equal(node.UID))

			records := getRecords()
			Expect(records).To(HaveLen(1))
			Expect(records[0].Name).To(Equal("nm-1"))
			Expect(records[0].UID).To(Equal("uid-1"))
			Expect(records[0].Reason).To(Equal("test reason"))
			Expect(records[0].CreatedBy).To(Equal("test-user"))
			Expect(records[0].Phase).To(Equal(v1beta1.MaintenanceSucceeded))
			Expect(records[0].DrainProgress).To(Equal(100))
			Expect(records[0].StartTime.Time.Unix()).To(Equal(nm.CreationTimestamp.Unix()))
			Expect(records[0].EndTime.Time.Unix()).To(Equal(end.Unix()))
		})
	})

	When("more maintenances of a node end", func() {
		It("should keep the newest records first and replace records of the same NodeMaintenance", func() {
			Expect(RecordEnded(ctx, cs, testNamespace, node, getTestNM("nm-1", "uid-1"), time.Now())).To(Succeed())
			Expect(RecordEnded(ctx, cs, testNamespace, node, getTestNM("nm-2", "uid-2"), time.Now())).To(Succeed())
			// a retried reconcile of the same NodeMaintenance
			Expect(RecordEnded(ctx, cs, testNamespace, node, getTestNM("nm-2", "uid-2"), time.Now())).To(Succeed())

			records := getRecords()
			Expect(records).To(HaveLen(2))
			Expect(records[0].Name).To(Equal("nm-2"))
			Expect(records[1].Name).To(Equal("nm-1"))
		})

		It("should keep at most MaxRecordsPerNode records", func() {
			for i := 0; i < MaxRecordsPerNode+5; i++ {
				nm := getTestNM(fmt.Sprintf("nm-%d", i), fmt.Sprintf("uid-%d", i))
				Expect(RecordEnded(ctx, cs, testNamespace, node, nm, time.Now())).To(Succeed())
			}
			records := getRecords()
			Expect(records).To(HaveLen(MaxRecordsPerNode))
			Expect(records[0].Name).To(Equal(fmt.Sprintf("nm-%d", MaxRecordsPerNode+4)))
		})
	})

	When("the node name is too long", func() {
		It("should not record the history", func() {
			node.Name = strings.Repeat("a", 250)
			_, ok := ConfigMapName(node.Name)
			Expect(ok).To(BeFalse())
			Expect(RecordEnded(ctx, cs, testNamespace, node, getTestNM("nm-1", "uid-1"), time.Now())).NotTo(Succeed())
		})
	})
})

func getTestNM(name, uid string) *v1beta1.NodeMaintenance {
	return &v1beta1.NodeMaintenance{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			UID:               types.UID(uid),
			Annotations:       map[string]string{v1beta1.CreatedByAnnotation: "test-user"},
			CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour)),
		},
		Spec: v1beta1.NodeMaintenanceSpec{
			NodeName: testNodeName,
			Reason:   "test reason",
		},
	}
}
