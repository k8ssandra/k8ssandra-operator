# Changelog

Changelog for the K8ssandra Operator, new PRs should update the `unreleased` section below with entries describing the changes like:

```markdown
* [CHANGE]
* [FEATURE]
* [ENHANCEMENT]
* [BUGFIX]
* [DOCS]
* [TESTING]
```

When cutting a new release, update the `unreleased` heading to the tag being generated and date, like `## vX.Y.Z - YYYY-MM-DD` and create a new placeholder section for  `unreleased` entries.

## unreleased

* [CHANGE] #1505 Replace yq Docker images with k8ssandra-client ones
* [CHANGE] [#1508](https://github.com/k8ssandra/k8ssandra-operator/issues/1508) Bump Medusa to 0.24.0

## v1.20.3 - 2024-10-18

* [CHANGE] Upgrade cass-operator's chart to 0.54.0 (v1.22.4) to allow overriding default image coordinates

## v1.20.2 - 2024-10-07

* [DOCS] [#1469](https://github.com/riptano/mission-control/issues/1469) Add docs for Reaper's Control Plane deployment mode
* [CHANGE] Bump default Medusa version to 0.22.3
* [BUGFIX] [#1409](https://github.com/k8ssandra/k8ssandra-operator/issues/1409) Vector would crash in the Cassandra log parsing if empty lines were present. Add automated tests for Vector parsing rules.
* [BUGFIX] [#1425](https://github.com/k8ssandra/k8ssandra-operator/issues/1425) prepare-helm-release.sh requires kustomize to be in the path and that makes make manifests fail.
* [BUGFIX] [#1422](https://github.com/k8ssandra/k8ssandra-operator/issues/1422) Changing a DC name should be forbidden

## v1.20.1 - 2024-09-19

* [BUGFIX] Upgrade cass-operator to v1.22.4 to fix security context overwrites

## v1.20.0 - 2024-09-18

* [BUGFIX] [#1399](https://github.com/k8ssandra/k8ssandra-operator/issues/1399) Fixed SecretSyncController to handle multiple namespaces
* [FEATURE] [#1382](https://github.com/k8ssandra/k8ssandra-operator/issues/1382) Add service to expose DC nodes in the control plane
* [FEATURE] [#1402](https://github.com/k8ssandra/k8ssandra-operator/issues/1402) Add support for readOnlyRootFilesystem
* [CHANGE] Upgrade cass-operator to v1.22.3
