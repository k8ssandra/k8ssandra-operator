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

* [ENHANCEMENT] [#1274](https://github.com/k8ssandra/k8ssandra-operator/issues/1274) On upgrade, do not modify the CassandraDatacenter object unless instructed with an annotation `k8ssandra.io/autoupdate-spec` with value `once` or `always`
