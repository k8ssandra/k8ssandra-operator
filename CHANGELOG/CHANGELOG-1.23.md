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

## v1.23.1 - 2025-05-29
* [BUGFIX] [#2121](https://github.com/riptano/mission-control/issues/2121) Add cluster's common L&A to replicated secrets and Vector's config map

## v1.23.0 - 2025-05-28

* [CHANGE] [#1542](https://github.com/k8ssandra/k8ssandra-operator/issues/1542) Support for Stargate has been deprecated and will be removed in future release.
* [CHANGE] []() Update k8ssandra-client to v0.7.0 to align with cass-operator v1.24.1
* [CHANGE] [#1519](https://github.com/k8ssandra/k8ssandra-operator/issues/1519) Update cass-operator to version 1.24.1, Kubernetes dependencies to 1.31.x series, controller-runtime dependencies
* [CHANGE] [#1544](https://github.com/k8ssandra/k8ssandra-operator/issues/1544) Make Reaper use management-api for the connection by default
* [ENHANCEMENT] Add support for watching multiple specific namespaces in cluster scope deployments
* [ENHANCEMENT] Add support for common annotations and labels in the helm chart
* [BUGFIX] [#1538](https://github.com/k8ssandra/k8ssandra-operator/issues/1538) Fix perNodeConfig init-container to mount Volume "tmp" for the readOnlyRootFilesystem clusters
* [BUGFIX] [#1550](https://github.com/k8ssandra/k8ssandra-operator/issues/1550) Make storageProvider and bucketName optional to accomodate the use of the MedusaConfiguration API
* [BUGFIX] [#2805](https://github.com/riptano/mission-control/issues/2085) Add missing propagation of common Labels and Annotations
* [TESTING] [#1532](https://github.com/k8ssandra/k8ssandra-operator/issues/1532) Update the base Cassandra version to 5.0.4 in tests
* [TESTING] [#955](https://github.com/k8ssandra/k8ssandra-operator/issues/955) Update to kustomize v5.6.0 and fix the tests to correctly render the templates
