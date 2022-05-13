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

## v1.1.0 2022-05-13
* [CHANGE] [#545](https://github.com/k8ssandra/k8ssandra-operator/pull/545) Update to cass-operator v1.11.0
* [CHANGE] [#310](https://github.com/k8ssandra/k8ssandra-operator/issues/310) Update to Go 1.17 and Kubernetes dependencies (incl. controller-runtime)
* [FEATURE] [#454](https://github.com/k8ssandra/k8ssandra-operator/pull/454) Remote restore support for Medusa
* [ENHANCEMENT] [#465](https://github.com/k8ssandra/k8ssandra-operator/issues/465) ⁃ Refactor config-builder JSON marshaling
* [ENHANCEMENT] [#507](https://github.com/k8ssandra/k8ssandra-operator/issues/507) Shared Cassandra cluster and datacenter fields are defined in a single struct
* [BUGFIX] [#447](https://github.com/k8ssandra/k8ssandra-operator/issues/447) Reconciliation doesn't finish when adding DC with more than 1 node
* [BUGFIX] [#465](https://github.com/k8ssandra/k8ssandra-operator/issues/465) ⁃ Java garbage collection properties cannot be configured correctly
* [TESTING] [#112](https://github.com/k8ssandra/k8ssandra-operator/issues/112) ⁃ Run e2e tests against arbitrary context names
* [TESTING] [#462](https://github.com/k8ssandra/k8ssandra-operator/issues/462) ⁃ Use yq to parse fixture files
* [TESTING] [#517](https://github.com/k8ssandra/k8ssandra-operator/issues/517) ⁃ Create make targets to prepare a cluster for e2e tests
* [TESTING] [#514](https://github.com/k8ssandra/k8ssandra-operator/issues/514) ⁃ Consider cert-manager a prerequisite for e2e tests
* [TESTING] [#515](https://github.com/k8ssandra/k8ssandra-operator/issues/515) ⁃ Consider ingress controller a prerequisite for e2e tests
* [TESTING] [#463](https://github.com/k8ssandra/k8ssandra-operator/issues/463) ⁃ Make fixtures cloud-ready
