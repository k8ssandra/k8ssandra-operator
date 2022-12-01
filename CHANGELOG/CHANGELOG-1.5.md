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

* [FEATURE] [#600](https://github.com/k8ssandra/k8ssandra-operator/issues/600) Disable secrets management and replication with the external secrets provider
* [ENHANCEMENT] [#525](https://github.com/k8ssandra/k8ssandra-operator/issues/525) Deep-merge cluster- and dc-level templates
* [TESTING] [#761](https://github.com/k8ssandra/k8ssandra-operator/issues/761) Stabilize integration and e2e tests
