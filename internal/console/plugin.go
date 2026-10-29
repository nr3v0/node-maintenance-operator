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

package console

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	consolev1 "github.com/openshift/api/console/v1"
)

const (
	// PluginName is the name of the console plugin, it must match the name in console-plugin/package.json
	// and the console.openshift.io/plugins CSV annotation
	PluginName = "node-maintenance-console-plugin"
	// ServiceName is the name of the console plugin Service and must match the name of the Service in /bundle/manifests!
	ServiceName = "node-maintenance-operator-console-plugin"
	// ServicePort is the port of the console plugin Service and must match the port of the Service in /bundle/manifests!
	ServicePort = 9443
)

//+kubebuilder:rbac:groups=console.openshift.io,resources=consoleplugins,verbs=get;list;watch;create;update;patch

// CreateOrUpdatePlugin creates or updates the ConsolePlugin resource of the node maintenance console plugin.
// The plugin's Deployment and Service are deployed by OLM.
func CreateOrUpdatePlugin(ctx context.Context, cl client.Client, namespace string, log logr.Logger) error {
	cp := getConsolePlugin(namespace)
	oldCP := &consolev1.ConsolePlugin{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(cp), oldCP); apierrors.IsNotFound(err) {
		if err := cl.Create(ctx, cp); err != nil {
			return fmt.Errorf("could not create console plugin: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("could not check for existing console plugin: %w", err)
	} else {
		oldCP.Spec = cp.Spec
		if err := cl.Update(ctx, oldCP); err != nil {
			return fmt.Errorf("could not update console plugin: %w", err)
		}
	}
	log.Info("successfully created / updated console plugin", "name", PluginName)
	return nil
}

func getConsolePlugin(namespace string) *consolev1.ConsolePlugin {
	return &consolev1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: PluginName,
		},
		Spec: consolev1.ConsolePluginSpec{
			DisplayName: "Node Maintenance",
			Backend: consolev1.ConsolePluginBackend{
				Type: consolev1.Service,
				Service: &consolev1.ConsolePluginService{
					Name:      ServiceName,
					Namespace: namespace,
					Port:      ServicePort,
					BasePath:  "/",
				},
			},
		},
	}
}
