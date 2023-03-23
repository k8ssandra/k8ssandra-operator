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

* [BUGFIX] [#914](https://github.com/k8ssandra/k8ssandra-operator/issues/914) Don't parse logs by default when Vector telemetry is enabled.
* [BUGFIX] [#916](https://github.com/k8ssandra/k8ssandra-operator/issues/916) Deprecate jmxInitContainerImage field.
* [DOCS] [#919](https://github.com/k8ssandra/k8ssandra-operator/issues/919) Improve the release process documentation.
