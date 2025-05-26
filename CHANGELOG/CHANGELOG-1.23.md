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

* [CHANGE] []() Update k8ssandra-client to v0.7.0 to align with cass-operator v1.24.0
* [CHANGE] [#1519](https://github.com/k8ssandra/k8ssandra-operator/issues/1519) Update cass-operator to version 1.24, Kubernetes dependencies to 1.31.x series, controller-runtime dependencies
* [ENCHANCEMENT] Add support for common annotations and labels in the helm chart
