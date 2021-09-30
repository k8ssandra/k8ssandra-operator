# Changelog

Changelog for the K8ssandra Operator, new PRs should update the `unreleased` section below with entries describing the changes like:

```markdown
* [CHANGE]
* [FEATURE]
* [ENHANCEMENT]
* [BUGFIX]
* [TESTING]
```

Test another changelog change

When cutting a new release, update the `unreleased` heading to the tag being generated and date, like `## vX.Y.Z - YYYY-MM-DD` and create a new placeholder section for  `unreleased` entries.

## Unreleased

## v1.0.0-alpha.1 - 2021-09-30

[FEATURE] [#98](https://github.com/k8ssandra/k8ssandra-operator/issues/98) Create Helm chart for the operator

[BUGFIX] [#97](https://github.com/k8ssandra/k8ssandra-operator/issues/97) Create replicated superuser secret before creating CassandraDatacenters

[FEATURE] [#87](https://github.com/k8ssandra/k8ssandra-operator/issues/87) Initial C* configuration exposure

[TESTING] [#86](https://github.com/k8ssandra/k8ssandra-operator/issues/86) Functional testing of k8ssandra-operator deployed Stargate APIs

[FEATURE] [#83](https://github.com/k8ssandra/k8ssandra-operator/issues/83) Add support for service account token authentication to remote clusters

[ENHANCEMENT] [#58](https://github.com/k8ssandra/k8ssandra-operator/issues/58) Multi-cluster secret management/distribution

[ENHANCEMENT] [#56](https://github.com/k8ssandra/k8ssandra-operator/issues/56) Support the deletion of deployed Stargate objects

[FEATURE] [#17](https://github.com/k8ssandra/k8ssandra-operator/issues/17) Create Stargate instance per rack by default

[FEATURE] [#12](https://github.com/k8ssandra/k8ssandra-operator/issues/12) K8ssandraCluster status should report status of CassandraDatacenter and Stargate objects

[FEATURE] [#11](https://github.com/k8ssandra/k8ssandra-operator/issues/11) Add support for managing datacenters in remote clusters

[FEATURE] [#3](https://github.com/k8ssandra/k8ssandra-operator/issues/3) Add initial support for managing CassandraDatacenters

[FEATURE] [#5](https://github.com/k8ssandra/k8ssandra-operator/issues/5) Create initial CRD and controller for Stargate
