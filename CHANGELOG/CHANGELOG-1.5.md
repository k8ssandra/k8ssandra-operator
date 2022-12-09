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

* [FEATURE] [#775](https://github.com/k8ssandra/k8ssandra-operator/issues/775) Add the ability to inject and configure a Vector agent sidecar in the Cassandra pods
* [FEATURE] [#600](https://github.com/k8ssandra/k8ssandra-operator/issues/600) Disable secrets management and replication with the external secrets provider
* [ENHANCEMENT] [#323](https://github.com/k8ssandra/k8ssandra/issues/323) Use Cassandra internals for JMX authentication
* [ENHANCEMENT] [#770](https://github.com/k8ssandra/k8ssandra-operator/issues/770) Update to Go 1.19 and Operator SDK 1.25.2
* [ENHANCEMENT] [#525](https://github.com/k8ssandra/k8ssandra-operator/issues/525) Deep-merge cluster- and dc-level templates
* [BUGFIX] [#726](https://github.com/k8ssandra/k8ssandra-operator/issues/726) Don't propagate Cassandra tolerations to the Stargate deployment
* [TESTING] [#761](https://github.com/k8ssandra/k8ssandra-operator/issues/761) Stabilize integration and e2e tests
* [ENHANCEMENT] [#781](https://github.com/k8ssandra/k8ssandra-operator/issues/781) Expose perNodeConfigInitContainer.Image through CRD
