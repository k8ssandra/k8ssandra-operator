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

## v1.24.0 - 2025-06-16

* [CHANGE] [#1560](https://github.com/k8ssandra/k8ssandra-operator/pull/1560) Upgrade cass-operator to v1.25.0 and allow making the ClusterRole/Binding resources optional in the helm chart
* [CHANGE] [#1523](https://github.com/riptano/k8ssandra-operator/issues/1523) Add support for Reaper v4 and use 4.0.0-beta3 as default tag 
* [CHANGE] [#1561](https://github.com/k8ssandra/k8ssandra-operator/issues/1561) Bump Medusa to 0.24.1
* [ENHANCEMENT] [#1575](https://github.com/k8ssandra/k8ssandra-operator/issues/1575) Make Medusa's encryption materials fully configurable
* [BUGFIX] [#1553](https://github.com/k8ssandra/k8ssandra-operator/pull/1553) Add cluster's common labels and annotations to replicated secrets and Vector's config map
* [BUGFIX] [#1558](https://github.com/k8ssandra/k8ssandra-operator/issues/1558) Authentication settings aren't applied correctly to cassandra.yaml
