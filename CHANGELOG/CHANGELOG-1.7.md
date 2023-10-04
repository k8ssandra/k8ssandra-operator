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

## v1.7.1 - 2023-07-24

* [BUGFIX] [#1023](https://github.com/k8ssandra/k8ssandra-operator/issues/1023) Ensure merged serverVersion passed to IsNewMetricsEndpointAvailable()

## v1.7.0 - 2023-06-05

* [CHANGE] [#991](https://github.com/k8ssandra/k8ssandra-operator/issues/991) If EncryptionStores and all the Keystore/Truststore passwords are not set, the operator will not touch the cassandra-yaml's encryption fields.
* [CHANGE] [#601](https://github.com/k8ssandra/k8ssandra-operator/issues/601) Add injection annotation to Cassandra and Reaper pods
* [ENHANCEMENT] [#965](https://github.com/k8ssandra/k8ssandra-operator/issues/965) Allow for ability to specify containers where webhook will mount secrets
* [ENHANCEMENT] [#932](https://github.com/k8ssandra/k8ssandra-operator/issues/932) Add ability to set variables to the secret-injection annotation. Supported are `POD_NAME`, `POD_NAMESPACE` and `POD_ORDINAL`. Also, changed JSON key from `secretName` to `name`
* [BUGFIX] [#914](https://github.com/k8ssandra/k8ssandra-operator/issues/914) Don't parse logs by default when Vector telemetry is enabled.
* [BUGFIX] [#916](https://github.com/k8ssandra/k8ssandra-operator/issues/916) Deprecate jmxInitContainerImage field.
* [BUGFIX] [#946](https://github.com/k8ssandra/k8ssandra-operator/issues/946) Fix medusaClient to fetch the pods from correct namespace
* [BUGFIX] [#940](https://github.com/k8ssandra/k8ssandra-operator/issues/940) Vector resource requirements are correctly set to the server-system-logger and also allow override of server-system-logger resource properties from containers.
* [BUGFIX] [#973](https://github.com/k8ssandra/k8ssandra-operator/issues/973) ReplicatedSecrets don't get updated when a DC is added in a different namespace but without context
* [BUGFIX] [#969](https://github.com/k8ssandra/k8ssandra-operator/issues/969) Inline tags in EmbeddedObjectMeta to work around YAML bug
* [DOCS] [#935](https://github.com/k8ssandra/k8ssandra-operator/issues/935) Add and document updated dashboards for the new metrics endpoint
* [DOCS] [#919](https://github.com/k8ssandra/k8ssandra-operator/issues/919) Improve the release process documentation.
* [CHANGE] [#601](https://github.com/k8ssandra/k8ssandra-operator/issues/601) Add injection annotation to Cassandra and Reaper pods
* [ENHANCEMENT] [#965](https://github.com/k8ssandra/k8ssandra-operator/issues/965) Allow for ability to specify containers where webhook will mount secrets
