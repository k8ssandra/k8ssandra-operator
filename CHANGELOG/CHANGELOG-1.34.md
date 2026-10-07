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

* [ENHANCEMENT] Add `affinity` and `topologySpreadConstraints` support to the k8ssandra-operator chart, allowing the operator pods to be spread across nodes when `replicaCount` is greater than 1
* [ENHANCEMENT] [#1796](https://github.com/k8ssandra/k8ssandra-operator/issues/1796) Declare JMX container port (7199) on the Cassandra pod template when enabling remote JMX access for Reaper
* [ENHANCEMENT] [#1754](https://github.com/k8ssandra/k8ssandra-operator/issues/1754) Use namespaceselector on the webhooks if watchNamespaces is used
* [BUGFIX] [#1788](https://github.com/k8ssandra/k8ssandra-operator/issues/1788) Don't stop syncing Medusa backups even if there's a requeue
* [TESTING] [#1804](https://github.com/k8ssandra/k8ssandra-operator/issues/1804) Slightly increase medusa-related timeouts in ITs
* [TESTING] [#1806](https://github.com/k8ssandra/k8ssandra-operator/issues/1806) Replace minio with silo
