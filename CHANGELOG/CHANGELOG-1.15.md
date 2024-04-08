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
* [CHANGE] [#1266](https://github.com/k8ssandra/k8ssandra-operator/issues/1266) Deprecate the usage of non-namespace-local MedusaConfigurations within a K8ssandraCluster, and throw an error from the webhook when users attempt to use one.