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

- [ENHANCEMENT] [#1094](https://github.com/k8ssandra/k8ssandra-operator/issues/1094) Expose AdditionalAnnotations field for cassDC.
- [ENHANCEMENT] [#1115](https://github.com/k8ssandra/k8ssandra-operator/issues/1115) Add a validation check for the projected pod names length
* [CHANGE] [#1050](https://github.com/k8ssandra/k8ssandra-operator/issues/1050) Remove unnecessary requeues in the Medusa controllers
