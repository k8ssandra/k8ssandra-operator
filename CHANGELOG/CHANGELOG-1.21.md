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

* [CHANGE] [#1441](https://github.com/k8ssandra/k8ssandra-operator/issues/1441) Use k8ssandra-client instead of k8ssandra-tools for CRD upgrades
* [BUGFIX] [#1383](https://github.com/k8ssandra/k8ssandra-operator/issues/1383) Do not create MedusaBackup if MedusaBakupJob did not fully succeed
* [ENHANCEMENT] [#1667](https://github.com/k8ssahttps://github.com/k8ssandra/k8ssandra/issues/1667) Add `skipSchemaMigration` option to `K8ssandraCluster.spec.reaper`
* [FEATURE] [#1508](https://github.com/riptano/mission-control/issues/1508) Make k8ssandra-operator deploy Reaper capable of interaction with encrypted mgmt-api
