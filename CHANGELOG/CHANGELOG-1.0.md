# Changelog

Changelog for the K8ssandra Operator, new PRs should update the `unreleased` section below with entries describing the changes like:

```markdown
* [CHANGE]
* [FEATURE]
* [ENHANCEMENT]
* [BUGFIX]
* [TESTING]
```

When cutting a new release, update the `unreleased` heading to the tag being generated and date, like `## vX.Y.Z - YYYY-MM-DD` and create a new placeholder section for  `unreleased` entries.

## Unreleased
* [FEATURE] [#45](https://github.com/k8ssandra/k8ssandra-operator/issues/45) Migrate the Medusa controllers to k8ssandra-operator
* [CHANGE] [#232](https://github.com/k8ssandra/k8ssandra-operator/issues/232) Use ReconcileResult in K8ssandraClusterReconciler
* [CHANGE] [#237](https://github.com/k8ssandra/k8ssandra-operator/issues/237) Add cass-operator dev kustomization deployment
* [ENHANCEMENT] [#218](https://github.com/k8ssandra/k8ssandra-operator/issues/218) Add `CassandraInitialized` status condition
* [ENHANCEMENT] [#24](https://github.com/k8ssandra/k8ssandra-operator/issues/24) Make Reaper images configurable and
  use same struct for both Reaper and Stargate images
* [ENHANCEMENT] [#136](https://github.com/k8ssandra/k8ssandra-operator/issues/136) Add shortNames for the K8ssandraCluster CRD
* [ENHANCEMENT] [#234](https://github.com/k8ssandra/k8ssandra-operator/issues/234) Add logic to configure and reconcile a Prometheus ServiceMonitor for each Cassandra Datacenter.
* [ENHANCEMENT] [#234](https://github.com/k8ssandra/k8ssandra-operator/issues/234) Add logic to configure and reconcile a Prometheus ServiceMonitor for each Stargate deployment.
* [CHANGE] [#136](https://github.com/k8ssandra/k8ssandra-operator/issues/136) Add shortNames for the K8ssandraCluster CRD
* [ENHANCEMENT] [#170](https://github.com/k8ssandra/k8ssandra-operator/issues/170) Enforce cluster-wide authentication by default 

## v1.0.0-alpha.2 - 2021-12-03

* [FEATURE] [#4](https://github.com/k8ssandra/k8ssandra-operator/issues/4) Add support for Reaper
* [FEATURE] [#15](https://github.com/k8ssandra/k8ssandra-operator/pull/15) Add finalizer for K8ssandraCluster
* [FEATURE] [#95](https://github.com/k8ssandra/k8ssandra-operator/issues/95) Cluster-scoped deployments
* [FEATURE] [#212](https://github.com/k8ssandra/k8ssandra-operator/issues/212) Allow management API heap size to be configured
* [ENHANCEMENT] [#210](https://github.com/k8ssandra/k8ssandra-operator/issues/210) Improve seeds handling
* [ENHANCEMENT] [#27](https://github.com/k8ssandra/k8ssandra-operator/issues/27) Make Reaper readiness and liveness 
  probes configurable
* [BUGFIX] [#203](https://github.com/k8ssandra/k8ssandra-operator/issues/203) Superuser secret name not set on CassandraDatacenters
* [BUGFIX] [#156](https://github.com/k8ssandra/k8ssandra-operator/issues/156) Stargate auth table creation may trigger a table ID mismatch

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
