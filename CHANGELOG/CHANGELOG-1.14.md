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
* [BUGFIX] [#1253](https://github.com/k8ssandra/k8ssandra-operator/issues/1253) Medusa storage secrets are now labelled with a unique label.
* [FEATURE] [#1260](https://github.com/k8ssandra/k8ssandra-operator/issues/1260) Update controller-gen to version 0.14.0.
* [BUGFIX] [#1240](https://github.com/k8ssandra/k8ssandra-operator/issues/1240) The PullSecretRef for medusa is ignored in the standalone deployment of medusa



## v1.14.0 - 2024-04-02

* [FEATURE] [#1242](https://github.com/k8ssandra/k8ssandra-operator/issues/1242) Allow for creation of replicated secrets with a prefix, so that we can distinguish between multiple secrets with the same origin but targeting different clusters.
* [BUGFIX] [#1226](https://github.com/k8ssandra/k8ssandra-operator/issues/1226) Medusa purge cronjob should be created in the operator namespace
* [BUGFIX] [#1141](https://github.com/k8ssandra/k8ssandra-operator/issues/1141) Use DC name override when naming secondary resources
* [BUGFIX] [#1138](https://github.com/k8ssandra/k8ssandra-operator/issues/1138) Use cluster name override for metrics agent ConfigMap 
* [BUGFIX] [#1252](https://github.com/k8ssandra/k8ssandra-operator/issues/1252) Sanitize DC name in pods selector
* [CHANGE] Update Medusa to v0.20.1
* [BUGFIX] [#1235](https://github.com/k8ssandra/k8ssandra-operator/issues/1235) Prevent operator from modifying the spec when superUserRef is not set. Also, remove the injection to mount secrets to cassandra container.
* [BUGFIX] [#1239](https://github.com/k8ssandra/k8ssandra-operator/issues/1239) Use `concurrent_transfers` setting from MedusaConfiguration object if it is actually set.