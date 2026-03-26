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

* [CHANGE] [1704](https://github.com/k8ssandra/k8ssandra-operator/issues/1704) Bump cass-operator to v1.30.0
* [CHANGE] [#1694](https://github.com/k8ssandra/k8ssandra-operator/issues/1694) Upgrade cass-operator to v1.29.0 and fix linter issues
* [FEATURE] [#1695](https://github.com/k8ssandra/k8ssandra-operator/issues/1695) Add new CRD option Config for Vector. This accepts the input as TOML and sets it at the server-system-logger's Vector instance. Used for global configuration of Vector.
* [ENHANCEMENT] [#1682](https://github.com/k8ssandra/k8ssandra-operator/issues/1682) Allow configuring the number of parallel rebuilds

