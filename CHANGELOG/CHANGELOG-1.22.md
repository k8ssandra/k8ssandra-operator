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

* [CHANGE] [#1499](https://github.com/k8ssandra/k8ssandra-operator/issues/1499) Bump cassandra-operator to v1.23.2 / 0.55.2 Helm chart
* [CHANGE] [#1505](https://github.com/k8ssandra/k8ssandra-operator/issues/1505) Replace yq Docker images with k8ssandra-client ones
* [CHANGE] [#1508](https://github.com/k8ssandra/k8ssandra-operator/issues/1508) Bump Medusa to 0.24.0
* [CHANGE] [#1516](https://github.com/k8ssandra/k8ssandra-operator/pull/1516) Add support for read-only file systems to Medusa containers
* [BUGFIX] [#1489](https://github.com/k8ssandra/k8ssandra-operator/issues/1489) Fix reconciliation of Medusa with config without credentials file.
