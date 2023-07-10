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

* [CHANGE] [#1005](https://github.com/k8ssandra/k8ssandra-operator/issues/1005) Support 7.x.x version numbers for DSE and 5.x.x for Cassandra
* [CHANGE] [#985](https://github.com/k8ssandra/k8ssandra-operator/issues/985) CI/CD does not produce images for some commits
* [CHANGE] [#483](https://github.com/k8ssandra/k8ssandra-operator/issues/483) Deploy a standalone Medusa pod for operator to Medusa direct interactions
* [CHANGE] Upgrade to Medusa v0.15.0
* [ENHANCEMENT] [#693](https://github.com/k8ssandra/k8ssandra-operator/issues/693) Build and publish arm64 images
* [ENHANCEMENT] [#842](https://github.com/k8ssandra/k8ssandra-operator/issues/842) Remove usages of deprecated created-by label
