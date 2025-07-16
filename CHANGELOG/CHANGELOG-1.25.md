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

* [CHANGE] [#1582](https://github.com/k8ssandra/k8ssandra-operator/issues/1582) Replace use of Endpoints with EndpointSlices, same as cass-operator v1.26.0
* [ENHANCEMENT] [#1578](https://github.com/k8ssandra/k8ssandra-operator/issues/1578) Disable webhooks installation for non admin installs
* [ENHANCEMENT] [#1575](https://github.com/k8ssandra/k8ssandra-operator/issues/1575) Make Medusa's encryption materials fully configurable
* [BUGFIX] [#1572](https://github.com/k8ssandra/k8ssandra-operator/issues/1572) Prevent K8ssandraCluster deletion until the CassDCs have been deleted effectively