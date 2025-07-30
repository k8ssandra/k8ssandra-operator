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

* [CHANGE] Upgrade Medusa to v0.25.0
* [CHANGE] Upgrade Reaper to 4.0.0-rc1
* [CHANGE] [#1588](https://github.com/k8ssandra/k8ssandra-operator/issues/1588) Use ubi9-micro as the base image for the operator
* [BUGFIX] Support endpointslices in OpenShift