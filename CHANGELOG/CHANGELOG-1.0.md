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

* [CHANGE] [#310](https://github.com/k8ssandra/k8ssandra-operator/issues/310) Update to Go 1.17 and Kubernetes dependencies (incl. controller-runtime)
* [TESTING] [#112](https://github.com/k8ssandra/k8ssandra-operator/issues/112) ⁃ Run e2e tests against arbitrary context names
* [TESTING] [#462](https://github.com/k8ssandra/k8ssandra-operator/issues/462) ⁃ Use yq to parse fixture files

## v1.0.1 2022-03-07

* [BUGFIX] [#447](https://github.com/k8ssandra/k8ssandra-operator/issues/447) Reconciliation doesn't finish when adding DC with more than 1 node

## v1.0.0  2022-02-17

* [CHANGE] [#423](https://github.com/k8ssandra/k8ssandra-operator/pull/423) Update to cass-operator v1.10.0
* [CHANGE] [#411](https://github.com/k8ssandra/k8ssandra-operator/issues/411) Rename `cassandraTelemetry` to just `telemetry`
* [CHANGE] [#417](https://github.com/k8ssandra/k8ssandra-operator/issues/417) Upgrade to Reaper 3.1.1
* [FEATURE] [#121](https://github.com/k8ssandra/k8ssandra-operator/issues/121) Add validating webhook for K8ssandraCluster
* [FEATURE] [#402](https://github.com/k8ssandra/k8ssandra-operator/issues/402) Expose CassandraDatacenter Tolerations property
* [BUGFIX] [#400](https://github.com/k8ssandra/k8ssandra-operator/issues/381) Keyspaces get altered incorrectly during DC migration
* [BUGFIX] [#381](https://github.com/k8ssandra/k8ssandra-operator/issues/381) Add additionalSeeds property to support migrations
* [BUGFIX] [#295](https://github.com/k8ssandra/k8ssandra-operator/issues/295) Check for schema agreement before applying schema changes
* [BUGFIX] [#421](https://github.com/k8ssandra/k8ssandra-operator/pull/421) Fix data plane kustomizations
* [DOCS] [#394](https://github.com/k8ssandra/k8ssandra-operator/issues/394) Fix examples in install guide
* [TESTING] [#332](https://github.com/k8ssandra/k8ssandra-operator/issues/332) Configure mocks to avoid panics
* [TESTING] [#407](https://github.com/k8ssandra/k8ssandra-operator/issues/407) Fix `nil` condition checks in backup/restore integration tests
* [TESTING] [#422](https://github.com/k8ssandra/k8ssandra-operator/pull/422) Fix single-dc Reaper e2e test

## v0.5.0 2022-02-10

* [CHANGE] [#120](https://github.com/k8ssandra/k8ssandra-operator/issues/120) Remove `systemLoggerResources` from K8ssandraCluster spec
* [FEATURE] [#296](https://github.com/k8ssandra/k8ssandra-operator/issues/296) Expose the stopped property from the
  CassandraDatacenter spec
* [BUGFIX] [#401](https://github.com/k8ssandra/k8ssandra-operator/issues/401) Use a LocalObjectReference for the bucket secret of Medusa 
* [BUGFIX] [#389](https://github.com/k8ssandra/k8ssandra-operator/issues/389) Configure watches for control plane client
* [BUGFIX] [#398](https://github.com/k8ssandra/k8ssandra-operator/issues/398) Removing a DC with size > 1 causes panic
* [TESTING] [#332](https://github.com/k8ssandra/k8ssandra-operator/issues/332) Configure mocks to avoid panics
* [TESTING] [#361](https://github.com/k8ssandra/k8ssandra-operator/issues/361) Reduce flakiness in Stargate API tests

## v0.4.0 2022-02-04

* [CHANGE] [#315](https://github.com/k8ssandra/k8ssandra-operator/issues/315) Explicity set `start_rpc: false`
* [CHANGE] [#308](https://github.com/k8ssandra/k8ssandra-operator/issues/308) Remove per-DC Reaper templates
* [FEATURE] [#293](https://github.com/k8ssandra/k8ssandra-operator/issues/293) Add support for encryption
* [FEATURE] [#21](https://github.com/k8ssandra/k8ssandra-operator/issues/21) Add datacenter to existing cluster
* [FEATURE] [#284](https://github.com/k8ssandra/k8ssandra-operator/issues/284) Remove datacenter from existing cluster
* [FEATURE] [#178](https://github.com/k8ssandra/k8ssandra-operator/issues/178) If ClientConfig is modified, automatically restart the k8ssandra-operator
* [ENHANCEMENT] [#308](https://github.com/k8ssandra/k8ssandra-operator/issues/308) Improve logic to compute Reaper
* [ENHANCEMENT] [#342](https://github.com/k8ssandra/k8ssandra-operator/issues/342) Expose `allowMultipleNodesPerWorker` property
 availability mode and remove Reaper DC template
* [BUGFIX] [#335](https://github.com/k8ssandra/k8ssandra-operator/issues/335) single-up Makefile target does not generate ClientConfig 
* [BUGFIX] [#285](https://github.com/k8ssandra/k8ssandra-operator/issues/285) Fix cluster-scoped kustomizations 
* [BUGFIX] [#329](https://github.com/k8ssandra/k8ssandra-operator/issues/329) Fix cass-operator version
* [DOCS] [#357](https://github.com/k8ssandra/k8ssandra-operator/issues/357) Set up encryption
* [TESTING] [#290](https://github.com/k8ssandra/k8ssandra-operator/issues/290) Update `DumpClusterInfo` method to include more details
* [TESTING] [#339](https://github.com/k8ssandra/k8ssandra-operator/issues/339) Fix artifact uploads

## v1.0.0-alpha.3 2022-01-23

* [CHANGE] [#232](https://github.com/k8ssandra/k8ssandra-operator/issues/232) Use ReconcileResult in K8ssandraClusterReconciler
* [CHANGE] [#237](https://github.com/k8ssandra/k8ssandra-operator/issues/237) Add cass-operator dev kustomization
* [FEATURE] [#276](https://github.com/k8ssandra/k8ssandra-operator/issues/276) Enable Reaper UI authentication
* [FEATURE] [#249](https://github.com/k8ssandra/k8ssandra-operator/issues/249) Expose all Cassandra yaml settings in a structured fashion in the k8c CRD
* [FEATURE] [#45](https://github.com/k8ssandra/k8ssandra-operator/issues/45) Migrate the Medusa controllers to k8ssandra-operator
* [ENHANCEMENT] [#218](https://github.com/k8ssandra/k8ssandra-operator/issues/218) Add `CassandraInitialized` status condition
* [ENHANCEMENT] [#24](https://github.com/k8ssandra/k8ssandra-operator/issues/24) Make Reaper images configurable and
  use same struct for both Reaper and Stargate images
* [ENHANCEMENT] [#136](https://github.com/k8ssandra/k8ssandra-operator/issues/136) Add shortNames for the K8ssandraCluster CRD
* [ENHANCEMENT] [#234](https://github.com/k8ssandra/k8ssandra-operator/issues/234) Add logic to configure and reconcile a Prometheus ServiceMonitor for each Cassandra Datacenter.
* [ENHANCEMENT] [#234](https://github.com/k8ssandra/k8ssandra-operator/issues/234) Add logic to configure and reconcile a Prometheus ServiceMonitor for each Stargate deployment.
* [ENHANCEMENT] [#170](https://github.com/k8ssandra/k8ssandra-operator/issues/170) Enforce cluster-wide authentication by default 
* [ENHANCEMENT] [#267](https://github.com/k8ssandra/k8ssandra-operator/issues/267) Use the K8ssandraCluster name as cluster name for CassandraDatacenter objects
* [ENHANCEMENT] [#297](https://github.com/k8ssandra/k8ssandra-operator/issues/297) Reconcile standalone Stargate auth schema
* [TESTING] [#299](https://github.com/k8ssandra/k8ssandra-operator/issues/299) Cluster-scoped e2e test failing

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
