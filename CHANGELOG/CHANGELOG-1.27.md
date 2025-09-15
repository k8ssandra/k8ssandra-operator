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

* [ENHANCEMENT] [#1591](https://github.com/k8ssandra/k8ssandra-operator/issues/1591) Remove the old medusa purge cronjob in favor of scheduled tasks
* [ENHANCEMENT] [#1245](https://github.com/k8ssandra/k8ssandra-operator/issues/1245) Ensure ReplicatedSecret targets are cleaned up correctly
* [BUGFIX] [#1603](https://github.com/k8ssandra/k8ssandra-operator/issues/1603) Fix the crd-upgrader to update cass-operator CRDs also as part of the Helm upgrade
* [BUGFIX] [#1610](https://github.com/k8ssandra/k8ssandra-operator/issues/1610) Replace all Medusa setControllerReference with setOwnerReference when targetting CassandraDatacenter objects
* [BUGFIX] [#1618](https://github.com/k8ssandra/k8ssandra-operator/issues/1618) If two datacenters with same clusterName were present in different namespaces, it was possible that the wrong one was picked when checking the status of the restore. 