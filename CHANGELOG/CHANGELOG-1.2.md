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

* [CHANGE] Update to Medusa v0.13.3
* [FEATURE] [#496](https://github.com/k8ssandra/k8ssandra-operator/issues/496) Allow overriding Cassandra cluster names
* [ENHANCEMENT] [#584](https://github.com/k8ssandra/k8ssandra-operator/issues/584) Make Medusa use the Cassandra data PV for restore downloads
* [ENHANCEMENT] [#354](https://github.com/k8ssandra/k8ssandra-operator/issues/354) Add encryption support to Medusa
* [BUGFIX] [#549](https://github.com/k8ssandra/k8ssandra-operator/issues/549) Releases branches should run e2e tests against target release version