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

## Unreleased

* [BUGFIX] Upgrade cass-operator to v1.22.4 to fix security context overwrites

## v1.20.0 - 2024-09-18

* [BUGFIX] [#1399](https://github.com/k8ssandra/k8ssandra-operator/issues/1399) Fixed SecretSyncController to handle multiple namespaces
* [FEATURE] [#1382](https://github.com/k8ssandra/k8ssandra-operator/issues/1382) Add service to expose DC nodes in the control plane
* [FEATURE] [#1402](https://github.com/k8ssandra/k8ssandra-operator/issues/1402) Add support for readOnlyRootFilesystem
* [CHANGE] Upgrade cass-operator to v1.22.3
