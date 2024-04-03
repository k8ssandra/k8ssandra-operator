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
* [FEATURE] [#1242](https://github.com/k8ssandra/k8ssandra-operator/issues/1242) Allow for creation of replicated secrets with a prefix, so that we can distinguish between multiple secrets with the same origin but targeting different clusters.
* [BUGFIX] [#1226](https://github.com/k8ssandra/k8ssandra-operator/issues/1226) Medusa purge cronjob should be created in the operator namespace
* [BUGFIX] [#1141](https://github.com/k8ssandra/k8ssandra-operator/issues/1141) Use DC name override when naming secondary resources
* [BUGFIX] [#1138](https://github.com/k8ssandra/k8ssandra-operator/issues/1138) Use cluster name override for metrics agent ConfigMap 
