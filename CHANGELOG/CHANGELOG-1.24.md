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

* [CHANGE] [#1560](https://github.com/k8ssandra/k8ssandra-operator/pull/1560) Upgrade cass-operator to v1.25.0 and allow making the ClusterRole/Binding resources optional in the helm chart
* [BUGFIX] [#2121](https://github.com/riptano/mission-control/issues/2121) Add cluster's common L&A to replicated secrets and Vector's config map
* [BUGFIX] [#1558](https://github.com/k8ssandra/k8ssandra-operator/issues/1558) Authentication settings aren't applied correctly to cassandra.yaml
