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

## V1.2.2

* [ENHANCEMENT] [#681](https://github.com/k8ssandra/k8ssandra-operator/issues/681) Update to Go 1.18, Kubernetes 1.24, Operator SDK 1.23, controller-gen 0.9.2, Kustomize 4.5.7, controller-runtime 0.12.2

## v1.2.1 - 2022-09-13

* [BUGFIX] [#675](https://github.com/k8ssandra/k8ssandra-operator/issues/675) K8ssandra is ignoring capital letters in the clusterName
* [BUGFIX] [#641](https://github.com/k8ssandra/k8ssandra-operator/issues/641) Reaper ServiceMonitor is not properly configured
* [BUGFIX] [#640](https://github.com/k8ssandra/k8ssandra-operator/issues/640) Fix NPEs when authentication is disabled
* [BUGFIX] [#567](https://github.com/k8ssandra/k8ssandra-operator/issues/567) Fix no seeds error when all the Cassandra pods of a DC get restarted at once

## v1.2.0 - 2022-07-22

* [CHANGE] Update to Reaper v3.2.0
* [CHANGE] Update to cass-operator v1.12.0
* [CHANGE] Update to Medusa v0.13.4
* [FEATURE] [#570](https://github.com/k8ssandra/k8ssandra-operator/issues/570) Add CDC integration to k8ssandra-operator
* [FEATURE] [#620](https://github.com/k8ssandra/k8ssandra-operator/issues/620) Enable injecting volumes in the Cassandra pods
* [FEATURE] [#569](https://github.com/k8ssandra/k8ssandra-operator/issues/569) Enable injecting containers and init containers into the Cassandra pods
* [FEATURE] [#496](https://github.com/k8ssandra/k8ssandra-operator/issues/496) Allow overriding Cassandra cluster names
* [FEATURE] [#460](https://github.com/k8ssandra/k8ssandra-operator/pull/460) Add support for scheduling backups.
* [FEATURE] [#531](https://github.com/k8ssandra/k8ssandra-operator/issues/531) Add support for JWT authentication in Stargate
* [ENHANCEMENT] [#578](https://github.com/k8ssandra/k8ssandra-operator/issues/578) Expose Reaper Metrics through a ServiceMonitor
* [ENHANCEMENT] [#553](https://github.com/k8ssandra/k8ssandra-operator/issues/531) Add resources config settings for Reaper and Medusa containers
* [ENHANCEMENT] [#573](https://github.com/k8ssandra/k8ssandra-operator/issues/573) Add the ability to set metrics filters in MCAC
* [ENHANCEMENT] [#584](https://github.com/k8ssandra/k8ssandra-operator/issues/584) Make Medusa use the Cassandra data PV for restore downloads
* [ENHANCEMENT] [#354](https://github.com/k8ssandra/k8ssandra-operator/issues/354) Add encryption support to Medusa
* [BUGFIX] [#549](https://github.com/k8ssandra/k8ssandra-operator/issues/549) Releases branches should run e2e tests against target release version
