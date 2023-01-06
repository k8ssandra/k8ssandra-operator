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
* [ENHANCEMENT] [#796](https://github.com/k8ssandra/k8ssandra-operator/issues/796) Enable smart token allocation by default for DSE
* [ENHANCEMENT] [#765](https://github.com/k8ssandra/k8ssandra-operator/issues/765) Add GitHub workflow to test various k8s versions
* [ENHANCEMENT] [#323](https://github.com/k8ssandra/k8ssandra/issues/323) Use Cassandra internals for JMX authentication
* [ENHANCEMENT] [#770](https://github.com/k8ssandra/k8ssandra-operator/issues/770) Update to Go 1.19 and Operator SDK 1.25.2
* [ENHANCEMENT] [#525](https://github.com/k8ssandra/k8ssandra-operator/issues/525) Deep-merge cluster- and dc-level templates
* [ENHANCEMENT] [#781](https://github.com/k8ssandra/k8ssandra-operator/issues/781) Expose perNodeConfigInitContainer.Image through CRD
* [BUGFIX] [#726](https://github.com/k8ssandra/k8ssandra-operator/issues/726) Don't propagate Cassandra tolerations to the Stargate deployment
* [BUGFIX] [#778](https://github.com/k8ssandra/k8ssandra-operator/issues/778) Fix Stargate deployments on k8s clusters using a custom domain name
* [TESTING] [#761](https://github.com/k8ssandra/k8ssandra-operator/issues/761) Stabilize integration and e2e tests
* [TESTING] [#799](https://github.com/k8ssandra/k8ssandra-operator/issues/799) Fix GHA CI failures with Kustomize install being rate limited
* [FEATURE] [#501](https://github.com/k8ssandra/k8ssandra-operator/issues/501) Allow configuring annotations and labels on services, statefulsets, deployments and pods
