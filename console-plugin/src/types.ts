import { K8sGroupVersionKind, K8sResourceCommon } from '@openshift-console/dynamic-plugin-sdk';

export const pluginName = 'node-maintenance-console-plugin';

export const nodeMaintenanceGroupVersionKind: K8sGroupVersionKind = {
  group: 'nodemaintenance.medik8s.io',
  version: 'v1beta1',
  kind: 'NodeMaintenance',
};

export const consolePluginGroupVersionKind: K8sGroupVersionKind = {
  group: 'console.openshift.io',
  version: 'v1',
  kind: 'ConsolePlugin',
};

// See CreatedByAnnotation in api/v1beta1/nodemaintenance_types.go
export const createdByAnnotation = 'nodemaintenance.medik8s.io/created-by';

export const getCreatedBy = (nm: NodeMaintenanceKind): string | undefined =>
  nm.metadata?.annotations?.[createdByAnnotation];

export type MaintenancePhase = 'Running' | 'Succeeded' | 'Failed';

// See api/v1beta1/nodemaintenance_types.go
export type NodeMaintenanceKind = K8sResourceCommon & {
  spec?: {
    nodeName?: string;
    reason?: string;
  };
  status?: {
    phase?: MaintenancePhase;
    drainProgress?: number;
    pendingPods?: string[];
    lastError?: string;
  };
};

// See internal/history/history.go
export const historyConfigMapNamePrefix = 'node-maintenance-history-';
export const historyDataKey = 'history';

export type MaintenanceRecord = {
  name: string;
  uid: string;
  reason?: string;
  createdBy?: string;
  startTime: string;
  endTime: string;
  phase?: MaintenancePhase;
  drainProgress?: number;
  lastError?: string;
};

export type ConsolePluginKind = K8sResourceCommon & {
  spec?: {
    backend?: {
      service?: {
        namespace?: string;
      };
    };
  };
};

export type ConfigMapKind = K8sResourceCommon & {
  data?: { [key: string]: string };
};

// isActiveNodeMaintenance returns whether the NodeMaintenance keeps its node in maintenance.
// A failed NodeMaintenance uncordons the node, and a deleted one is ending the maintenance.
export const isActiveNodeMaintenance = (nm: NodeMaintenanceKind, nodeName: string): boolean =>
  nm.spec?.nodeName === nodeName && !nm.metadata?.deletionTimestamp && nm.status?.phase !== 'Failed';
