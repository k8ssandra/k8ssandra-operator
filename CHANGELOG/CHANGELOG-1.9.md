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

* [CHANGE] [#1037](https://github.com/k8ssandra/k8ssandra-operator/issues/1037) Upgrade Medusa to v0.16.1
* [CHANGE] Upgrade to cass-operator v1.17.1
* [ENHANCEMENT] [#1045](https://github.com/k8ssandra/k8ssandra-operator/issues/1045) Validate MedusaBackup before doing restore to prevent data loss scenarios
* [ENHANCEMENT] [#1046](https://github.com/k8ssandra/k8ssandra-operator/issues/1046) Add detailed backup information in the MedusaBackup CRD status
* [BUGFIX] [#1027](https://github.com/k8ssandra/k8ssandra-operator/issues/1027) Point system-logger image to use the v1.16.0 tag instead of latest
* [BUGFIX] [#1026](https://github.com/k8ssandra/k8ssandra-operator/issues/1026) Fix DC name overrides not being properly handled
* [BUGFIX] [#981](https://github.com/k8ssandra/k8ssandra-operator/issues/981) Fix race condition in K8ssandraTask status update
* [BUGFIX] [#1023](https://github.com/k8ssandra/k8ssandra-operator/issues/1023) Ensure merged serverVersion passed to IsNewMetricsEndpointAvailable()
* [BUGFIX] [#1017](https://github.com/k8ssandra/k8ssandra-operator/issues/1017) Fix MedusaTask to delete only its own backups
* [DOCS] [#1019](https://github.com/k8ssandra/k8ssandra-operator/issues/1019) Add PR review guidelines documentation
* [TESTING] [#1037](https://github.com/k8ssandra/k8ssandra-operator/issues/1037) Add non regression test for Medusa issue with scale up after a restore
