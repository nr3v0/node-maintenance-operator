# Node Maintenance Console Plugin

OpenShift Console dynamic plugin which shows nodes that are in maintenance by the Node Maintenance Operator.

It requires OpenShift 4.19+ (PatternFly 6). It contributes a `console.node/status` extension, so nodes with an active `NodeMaintenance`
show an additional **In Maintenance** status, next to **Scheduling disabled**, under **Compute → Nodes** and
on the Node details page. The status popover shows the `NodeMaintenance`, its reason, phase and drain progress.

It also adds a **Maintenances** tab to the Node details page (`console.tab/horizontalNav`), listing the active
`NodeMaintenance` and the node's last 20 ended maintenances, including their requester (the user who created the `NodeMaintenance`). Since a `NodeMaintenance` is deleted to end the
maintenance, the operator records ended maintenances in the `node-maintenance-history-<node>` ConfigMap in its
namespace (`internal/history`).

## Deployment

The plugin's Deployment and Service are part of the operator bundle (`config/optional/console-plugin`),
and the operator creates the `ConsolePlugin` resource on OpenShift (`internal/console/plugin.go`).
Enable the plugin when installing the operator, or later with:

```bash
oc patch consoles.operator.openshift.io cluster --type=json \
  -p '[{"op": "add", "path": "/spec/plugins/-", "value": "node-maintenance-console-plugin"}]'
```

## Development

```bash
npm ci
npm run lint
npm run build
```

Build the image from the repository root with `make console-plugin-build`.

`make dev-olm-deploy` builds only the operator image, so the bundle references the released plugin image.
To deploy the plugin from source as well, build and push it next to the operator:

```bash
make dev-olm-deploy CONSOLE_PLUGIN_IMG=ttl.sh/medik8s-node-maintenance-console-$(uuidgen | cut -c1-8):2h \
  DEV_OLM_EXTRA_BUILD_TARGETS="console-plugin-build console-plugin-push"
```
