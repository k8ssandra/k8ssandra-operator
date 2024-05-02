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

* [BUGFIX] [#1272](https://github.com/k8ssandra/k8ssandra-operator/issues/1272) Prevent cass-operator from creating users when an external DC is referenced to allow migration through expansion
* [ENHANCEMENT] [#1066](https://github.com/k8ssandra/k8ssandra-operator/issues/1066) Remove the medusa standalone pod as it is not needed anymore