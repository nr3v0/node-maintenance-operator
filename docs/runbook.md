# Node Maintenance Operator (NMO) — Operational runbook

## Purpose

Commands and fields for **NodeMaintenance** day-2 ops. Behaviour details: **`../ARCHITECTURE.md`**, **`failure_modes.md`**.

## Components

| Piece | Role |
|-------|------|
| **Operator deployment** | Reconciles **`NodeMaintenance`**, serves the **mutating** and **validating webhooks**. |
| **CRD** | **`nodemaintenance.medik8s.io/v1beta1`**, kind **`NodeMaintenance`**, short **`nm`**, **cluster-scoped**. |

## Operator process (observability)

| Topic | Note |
|-------|------|
| **Health / readiness** | Default probe bind **`:8081`** (`cmd/main.go`). |
| **Metrics** | **`--metrics-bind-address`** default **`:8443`**; secured (TLS + Kubernetes bearer-token authn/authz via controller-runtime filters) — same pattern as NHC/SNR. **`nmo_node_in_maintenance{node,nodemaintenance,reason}`** is **1** per node in maintenance. |
| **Webhook** | TLS under **`/apiserver.local.config/certificates`** when OLM injects certs; port **9443** default. On OpenShift, TLS settings (both metrics and webhook) also follow the cluster's **`APIServer`** TLS profile, and the process restarts if that profile changes. |
| **Leader election** | **`--leader-elect`** defaults **false** in code; CSV may override. ID **`135b1886.medik8s.io`**. |

## Prerequisites

| Requirement | Note |
|-------------|------|
| **RBAC** | Operator SA can **patch nodes**, **evict pods**, manage **leases**, list **apps** workloads (for drain). |
| **OpenShift** | Control-plane **NodeMaintenance** create requires **etcd** disruption allowed (webhook). |
| **Single CR per node** | Enforced at admission. |

## Quick checks

```bash
kubectl get nodemaintenance
kubectl get nm
kubectl describe nm <name>
kubectl get node <nodeName> -o yaml
kubectl get leases.coordination.k8s.io -A | grep -i maintenance || true
```

Inspect **taints** and **labels** on the node:

```bash
kubectl get node <nodeName> -o jsonpath='{.spec.taints}{"\n"}{.metadata.labels.remediation\.medik8s\.io/exclude-from-remediation}{"\n"}'
```

## What to inspect on **NodeMaintenance**

| Area | Look for |
|------|----------|
| **`spec.nodeName`**, **`reason`** | Target and intent. |
| **`nodemaintenance.medik8s.io/created-by`** annotation | User who created the CR, set by the mutating webhook; can't be changed. |
| **`status.phase`** | **Running** → **Succeeded** or **Failed**. |
| **`status.drainProgress`** | Progress during eviction retries. |
| **`status.pendingPods` / `pendingPodsRefs`** | Work still on the node. |
| **`status.lastError`** | Last reconcile error text. |
| **`status.errorOnLeaseCount`** | Approaching **>3** → risk of **Failed** + uncordon. |
| **Events** | **BeginMaintenance**, **SucceedMaintenance**, **FailedMaintenance**, **RemovedMaintenance**, **UncordonNode**. |

## What happens on the **Node** during maintenance

| Effect | Note |
|--------|------|
| **Taints** | **`node.kubernetes.io/unschedulable`** and **`medik8s.io/drain`** (**NoSchedule**). |
| **Cordon** | **`spec.unschedulable`** via drain helper. |
| **Label** | **`remediation.medik8s.io/exclude-from-remediation=true`**. |

### OpenShift: MachineConfigPool shows **Updating**

While a node is in maintenance, its **MachineConfigPool** reports **`UPDATED=False`** / **`UPDATING=True`** ("All nodes are updating to rendered-…"), although no MachineConfig changed. This is expected: the **Machine Config Operator** counts only schedulable nodes as ready, so a cordoned node makes the pool look like a rollout is in progress.

| Signal | During maintenance |
|--------|--------------------|
| **`UPDATEDMACHINECOUNT`** | Unchanged, equals **`MACHINECOUNT`**: the node is still on the pool's rendered config. |
| **`READYMACHINECOUNT`** | One less per node in maintenance. |
| **Node annotations** | **`machineconfiguration.openshift.io/currentConfig`** = **`desiredConfig`**, **`state=Done`**. |
| **Condition change** | **`Updating=True`** within seconds of the **`NodeMaintenance`** creation. |

Why (MCO **`pkg/controller/common/layered_node_state.go`** and **`pkg/controller/node/status.go`**):

- **`CheckNodeReady`** returns an error when **`node.spec.unschedulable`** is true, so the node is neither *ready* nor *available* (**`IsUnavailableForUpdate`**).
- The pool is **Updated** only if **all** nodes are updated **and** ready **and** none is unavailable; otherwise an unpaused pool gets **`Updating=True`**.

The MCO cordons nodes itself while applying MachineConfigs, so it treats any cordoned node as being updated. Cordoning a node by hand has the same effect.

**Impact:**

- The cordoned node uses up the pool's **`maxUnavailable`** (default **1**). A **new** MachineConfig change for that pool, or the pool's part of a **cluster upgrade**, waits until the maintenance ends.
- The console **Overview** may show the pool as updating.

**Resolve:** nothing to do. NMO can't avoid this without not cordoning. Deleting the **`NodeMaintenance`** uncordons the node and the pool returns to **`Updated=True`**. Plan maintenances and MachineConfig changes or upgrades so they don't overlap, or raise the pool's **`maxUnavailable`** if both must run at once.

To confirm the pool isn't actually rolling out a new config:

```bash
oc get mcp
oc get node <nodeName> -o jsonpath='{.metadata.annotations.machineconfiguration\.openshift\.io/currentConfig}{"\n"}{.metadata.annotations.machineconfiguration\.openshift\.io/desiredConfig}{"\n"}{.metadata.annotations.machineconfiguration\.openshift\.io/state}{"\n"}'
```

## Alerts

### NodeInMaintenance

| | |
|---|---|
| **Severity** | **warning** (OpenShift platform alert, fires within about **20s**: no pending period, **10s** scrape and rule evaluation intervals) |
| **Meaning** | A **`NodeMaintenance`** CR for the node is **Running** or **Succeeded**: the node is cordoned, tainted, drained and excluded from automated remediation. The description links to the node and shows the **reason**; the **`NodeMaintenance`** name is in the **`nodemaintenance`** alert label. |
| **Impact** | Reduced cluster capacity; no new workloads are scheduled on the node. Expected during planned maintenance. |
| **Resolve** | When the maintenance is done, end it with **`oc delete nm <nodemaintenance>`**. The node is uncordoned and the alert clears. |

```bash
oc get nm -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName,REASON:.spec.reason,PHASE:.status.phase
```

If the alert doesn't fire on OpenShift, check that the operator namespace has the **`openshift.io/cluster-monitoring=true`** label and that the **ServiceMonitor** target is up (**Observe → Targets**).

## Console

On OpenShift, the **node-maintenance-console-plugin** shows **In Maintenance** for nodes under maintenance (**Compute → Nodes** and the Node details page). The Node details page has a **Maintenances** tab with the active maintenance and the last **20** ended ones, read from the ConfigMap **`node-maintenance-history-<node>`** in the operator namespace:

```bash
oc get cm -n <operator-namespace> -l nodemaintenance.medik8s.io/history=true
oc get cm -n <operator-namespace> node-maintenance-history-<nodeName> -o jsonpath='{.data.history}' | jq
```

If the plugin is missing, check that it is enabled (**`oc get consoles.operator.openshift.io cluster -o jsonpath='{.spec.plugins}'`**) and that its pod in the operator namespace is running.

## Debugging checklist

1. **Webhook** blocking create? — node exists, no duplicate **nm**, OpenShift **etcd** guard for masters. Both webhooks (mutating + validating) fail closed, so a down operator blocks creating **nm**.  
2. **Lease** errors? — **`leases.coordination.k8s.io`**, competing operators.  
3. **Drain** looping? — PDBs, **forbidden** evictions, terminating pods, **30s** timeout.  
4. **Stuck terminating CR?** — Operator logs, finalizer, RBAC **update** on **NodeMaintenance**.  
5. **MachineConfigPool Updating?** — expected while a node is cordoned, see **OpenShift: MachineConfigPool shows Updating** above.  

## Related pieces

- **`code_map.md`** — source file index.
