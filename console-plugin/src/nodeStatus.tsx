import * as React from 'react';
import { useTranslation } from 'react-i18next';
import { NodeKind, ResourceLink, WatchK8sResults } from '@openshift-console/dynamic-plugin-sdk';
import {
  getCreatedBy,
  isActiveNodeMaintenance,
  nodeMaintenanceGroupVersionKind,
  NodeMaintenanceKind,
  pluginName,
} from './types';

type NodeMaintenanceResources = {
  nodeMaintenances: NodeMaintenanceKind[];
};

type MaintenancePopoverProps = {
  node: NodeKind;
  resources: WatchK8sResults<NodeMaintenanceResources>;
};

// getActiveNodeMaintenance returns the NodeMaintenance which keeps the node in maintenance
const getActiveNodeMaintenance = (
  node: NodeKind,
  resources: WatchK8sResults<NodeMaintenanceResources>,
): NodeMaintenanceKind | undefined => {
  const nodeMaintenances = resources?.nodeMaintenances;
  if (!nodeMaintenances?.loaded || nodeMaintenances.loadError) {
    return undefined;
  }
  return (nodeMaintenances.data ?? []).find((nm) =>
    isActiveNodeMaintenance(nm, node.metadata?.name),
  );
};

export const isNodeInMaintenance = (
  node: NodeKind,
  resources: WatchK8sResults<NodeMaintenanceResources>,
): boolean => !!getActiveNodeMaintenance(node, resources);

// The node status title is static, so there is one "Draining <n>%" status extension per drain progress value,
// generated in webpack.config.ts. Each one watches the NodeMaintenances under the resource key
// `${drainProgressResourcePrefix}<n>`, which tells isDrainingAt the progress it's responsible for.
export const drainProgressResourcePrefix = 'drainProgress';

export const isDrainingAt = (
  node: NodeKind,
  resources: WatchK8sResults<{ [key: string]: NodeMaintenanceKind[] }>,
): boolean => {
  const key = Object.keys(resources ?? {}).find((k) => k.startsWith(drainProgressResourcePrefix));
  const nodeMaintenances = key && resources[key];
  if (!nodeMaintenances?.loaded || nodeMaintenances.loadError) {
    return false;
  }
  const progress = Number(key.slice(drainProgressResourcePrefix.length));
  const nm = (nodeMaintenances.data ?? []).find((n) => isActiveNodeMaintenance(n, node.metadata?.name));
  // the drain is complete when the maintenance succeeded
  return !!nm && nm.status?.phase !== 'Succeeded' && (nm.status?.drainProgress ?? 0) === progress;
};

// The "In Maintenance" status popover already shows the drain progress
export const DrainProgressPopover: React.FC = () => null;

export const MaintenancePopover: React.FC<MaintenancePopoverProps> = ({ node, resources }) => {
  const { t } = useTranslation(`plugin__${pluginName}`);
  const nm = getActiveNodeMaintenance(node, resources);
  if (!nm) {
    return null;
  }

  const phase = nm.status?.phase;
  let phaseText = t('Starting maintenance');
  if (phase === 'Succeeded') {
    phaseText = t('Node is in maintenance');
  } else if (phase === 'Running') {
    phaseText = t('Maintenance in progress');
  }

  return (
    <div>
      <p>
        <strong>{phaseText}</strong>
      </p>
      <p>
        {t(
          'The node is cordoned and drained. No new workloads are scheduled on it and it is excluded from automated remediation until the NodeMaintenance is deleted.',
        )}
      </p>
      <dl>
        <dt>
          <strong>{t('NodeMaintenance')}</strong>
        </dt>
        <dd>
          <ResourceLink groupVersionKind={nodeMaintenanceGroupVersionKind} name={nm.metadata?.name} />
        </dd>
        <dt>
          <strong>{t('Reason')}</strong>
        </dt>
        <dd>{nm.spec?.reason || '-'}</dd>
        <dt>
          <strong>{t('Requester')}</strong>
        </dt>
        <dd>{getCreatedBy(nm) || '-'}</dd>
        <dt>
          <strong>{t('Phase')}</strong>
        </dt>
        <dd>{phase || t('Unknown')}</dd>
        <dt>
          <strong>{t('Drain progress')}</strong>
        </dt>
        <dd>{nm.status?.drainProgress ?? 0}%</dd>
        {nm.status?.pendingPods?.length > 0 && (
          <>
            <dt>
              <strong>{t('Pending pods')}</strong>
            </dt>
            <dd>{nm.status.pendingPods.length}</dd>
          </>
        )}
      </dl>
    </div>
  );
};
