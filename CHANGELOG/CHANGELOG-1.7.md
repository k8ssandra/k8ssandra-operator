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

* [ENHANCEMENT] [#932](https://github.com/k8ssandra/k8ssandra-operator/issues/932) Add ability to set variables to the secret-injection annotation. Supported are `POD_NAME`, `POD_NAMESPACE` and `POD_ORDINAL`. Also, changed JSON key from `secretName` to `name`
* [BUGFIX] [#914](https://github.com/k8ssandra/k8ssandra-operator/issues/914) Don't parse logs by default when Vector telemetry is enabled.
* [BUGFIX] [#916](https://github.com/k8ssandra/k8ssandra-operator/issues/916) Deprecate jmxInitContainerImage field.
* [BUGFIX] [#940](https://github.com/k8ssandra/k8ssandra-operator/issues/940) Vector resource requirements are correctly set to the server-system-logger and also allow override of server-system-logger resource properties from containers.
* [DOCS] [#935](https://github.com/k8ssandra/k8ssandra-operator/issues/935) Add and document updated dashboards for the new metrics endpoint
* [DOCS] [#919](https://github.com/k8ssandra/k8ssandra-operator/issues/919) Improve the release process documentation.
