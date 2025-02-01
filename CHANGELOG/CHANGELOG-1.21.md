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

* [CHANGE] [#1450](https://github.com/k8ssandra/k8ssandra-operator/issues/1450) Update datacenter labels to use Kubernetes resource names for CassandraDatacenter, not the cleaned override name. Update to cass-operator 1.23.0
* [CHANGE] [#1441](https://github.com/k8ssandra/k8ssandra-operator/issues/1441) Use k8ssandra-client instead of k8ssandra-tools for CRD upgrades
* [ENHANCEMENT] [#1667](https://github.com/k8ssahttps://github.com/k8ssandra/k8ssandra/issues/1667) Add `skipSchemaMigration` option to `K8ssandraCluster.spec.reaper`
* [FEATURE] [#1034](https://github.com/k8ssandra/k8ssandra-operator/issues/1034) Add support for priorityClassName
* [BUGFIX] [#1454](https://github.com/k8ssandra/k8ssandra-operator/issues/1454) Do not try to work out backup status if there are no pods
* [BUGFIX] [#1383](https://github.com/k8ssandra/k8ssandra-operator/issues/1383) Do not create MedusaBackup if MedusaBakupJob did not fully succeed
* [BUGFIX] [#1460](https://github.com/k8ssandra/k8ssandra-operator/issues/1460) Fix podName calculations in medusa's hostmap.go to account for unbalanced racks also
* [BUGFIX] [#1466](https://github.com/k8ssandra/k8ssandra-operator/issues/1466) Do not overwrite existing status fields or forget to write the changes. Also, add new ContextName for the Datacenter to know where it used to be. 
* [ENHANCEMENT] [#1455](https://github.com/k8ssandra/k8ssandra-operator/issues/1455) Expose configuration of Medusa's gRPC server port
* [BUGFIX] [#1471](https://github.com/k8ssandra/k8ssandra-operator/issues/1471) Use namespaced service name when registering k8ssandra cluster to Reaper
* [BUGFIX] Upgrade cass-operator helm chart to 0.55.0 (1.23.0)
