import * as React from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  EmptyState,
  EmptyStateBody,
  Label,
  PageSection,
  Spinner,
  Title,
} from '@patternfly/react-core';
import { Table, Tbody, Td, Th, Thead, Tr } from '@patternfly/react-table';
import {
  K8sResourceCommon,
  ResourceLink,
  Timestamp,
  useK8sWatchResource,
} from '@openshift-console/dynamic-plugin-sdk';
import {
  ConfigMapKind,
  consolePluginGroupVersionKind,
  ConsolePluginKind,
  getCreatedBy,
  historyConfigMapNamePrefix,
  historyDataKey,
  isActiveNodeMaintenance,
  MaintenanceRecord,
  nodeMaintenanceGroupVersionKind,
  NodeMaintenanceKind,
  pluginName,
} from './types';

// see MaxRecordsPerNode in internal/history/history.go
const maxRecordsPerNode = 20;

// the last phase of an ended NodeMaintenance; Running means it was deleted before the drain completed
const phaseColor = (phase?: string) => {
  switch (phase) {
    case 'Succeeded':
      return 'green';
    case 'Failed':
      return 'red';
    case 'Running':
      return 'orange';
    default:
      return 'blue';
  }
};

const durationText = (startTime?: string, endTime?: string): string => {
  if (!startTime || !endTime) {
    return '-';
  }
  const seconds = Math.max(0, Math.round((Date.parse(endTime) - Date.parse(startTime)) / 1000));
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) {
    return `${days}d ${hours}h`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${seconds}s`;
};

const parseRecords = (cm: ConfigMapKind | undefined): MaintenanceRecord[] => {
  const data = cm?.data?.[historyDataKey];
  if (!data) {
    return [];
  }
  try {
    const records = JSON.parse(data);
    return Array.isArray(records) ? records : [];
  } catch {
    return [];
  }
};

/** "Maintenances" tab on the Node details page: maintenance state + maintenance history. */
const MaintenancesTab: React.FC<{ obj: K8sResourceCommon }> = ({ obj: node }) => {
  const { t } = useTranslation(`plugin__${pluginName}`);
  const nodeName = node.metadata?.name ?? '';

  const [nms, , nmsError] = useK8sWatchResource<NodeMaintenanceKind[]>({
    groupVersionKind: nodeMaintenanceGroupVersionKind,
    isList: true,
  });

  // the history ConfigMap lives in the operator namespace, which is the namespace of the plugin's Service
  const [consolePlugin, , pluginError] = useK8sWatchResource<ConsolePluginKind>({
    groupVersionKind: consolePluginGroupVersionKind,
    name: pluginName,
  });
  const operatorNamespace = consolePlugin?.spec?.backend?.service?.namespace;

  const [historyConfigMap, historyLoaded, historyError] = useK8sWatchResource<ConfigMapKind>(
    operatorNamespace
      ? {
          groupVersionKind: { version: 'v1', kind: 'ConfigMap' },
          namespace: operatorNamespace,
          name: `${historyConfigMapNamePrefix}${nodeName}`,
        }
      : null,
  );
  // a node without ended maintenances has no history ConfigMap
  const historyNotFound = historyError?.response?.status === 404;
  const loadError = pluginError || (historyError && !historyNotFound);
  const loaded = historyLoaded || historyNotFound;

  const nm = (nms ?? []).find((n) => isActiveNodeMaintenance(n, nodeName));
  const records = parseRecords(historyConfigMap);

  return (
    <PageSection>
      <Title headingLevel="h2" style={{ marginBottom: 'var(--pf-t--global--spacer--md)' }}>
        {t('Maintenance')}
      </Title>
      {nmsError ? (
        <Alert variant="warning" isInline title={t('Node Maintenance Operator not available')} />
      ) : nm ? (
        <Alert
          variant={nm.status?.phase === 'Succeeded' ? 'success' : 'info'}
          isInline
          title={
            nm.status?.phase === 'Succeeded'
              ? t('Node is in maintenance. Delete the NodeMaintenance to end the maintenance.')
              : t('Maintenance in progress: {{progress}}% drained', {
                  progress: nm.status?.drainProgress ?? 0,
                })
          }
        >
          <ResourceLink
            groupVersionKind={nodeMaintenanceGroupVersionKind}
            name={nm.metadata?.name}
            inline
          />{' '}
          {nm.spec?.reason}
          {getCreatedBy(nm) && (
            <div>{t('Requester: {{user}}', { user: getCreatedBy(nm) })}</div>
          )}
        </Alert>
      ) : (
        <Alert variant="info" isInline title={t('Node is not in maintenance.')} />
      )}

      <Title
        headingLevel="h2"
        style={{ margin: 'var(--pf-t--global--spacer--lg) 0 var(--pf-t--global--spacer--md)' }}
      >
        {t('Maintenance history')}
      </Title>
      {loadError ? (
        <Alert variant="danger" isInline title={t('Unable to load the maintenance history')} />
      ) : !loaded ? (
        <Spinner size="lg" />
      ) : records.length === 0 ? (
        <EmptyState titleText={t('No maintenances')} headingLevel="h3">
          <EmptyStateBody>{t('No maintenance of this node has ended yet.')}</EmptyStateBody>
        </EmptyState>
      ) : (
        <>
          <Table aria-label={t('Maintenance history')} variant="compact">
            <Thead>
              <Tr>
                <Th>{t('Name')}</Th>
                <Th>{t('Phase')}</Th>
                <Th>{t('Reason')}</Th>
                <Th>{t('Requester')}</Th>
                <Th>{t('Message')}</Th>
                <Th>{t('Started')}</Th>
                <Th>{t('Ended')}</Th>
                <Th>{t('Duration')}</Th>
              </Tr>
            </Thead>
            <Tbody>
              {records.map((r) => (
                <Tr key={r.uid}>
                  <Td dataLabel={t('Name')}>{r.name}</Td>
                  <Td dataLabel={t('Phase')}>
                    <Label color={phaseColor(r.phase)}>
                      {r.phase === 'Running' ? t('Ended while draining') : r.phase ?? '-'}
                    </Label>
                  </Td>
                  <Td dataLabel={t('Reason')}>{r.reason || '-'}</Td>
                  <Td dataLabel={t('Requester')}>{r.createdBy || '-'}</Td>
                  <Td dataLabel={t('Message')}>
                    {r.phase !== 'Succeeded' && r.lastError ? r.lastError : '-'}
                  </Td>
                  <Td dataLabel={t('Started')}>
                    {r.startTime ? <Timestamp timestamp={r.startTime} /> : '-'}
                  </Td>
                  <Td dataLabel={t('Ended')}>
                    {r.endTime ? <Timestamp timestamp={r.endTime} /> : '-'}
                  </Td>
                  <Td dataLabel={t('Duration')}>{durationText(r.startTime, r.endTime)}</Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
          <p style={{ marginTop: 'var(--pf-t--global--spacer--sm)' }}>
            {t('The last {{count}} ended maintenances are kept.', { count: maxRecordsPerNode })}
          </p>
        </>
      )}
    </PageSection>
  );
};

export default MaintenancesTab;
