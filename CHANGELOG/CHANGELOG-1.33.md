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

* [CHANGE] Update cass-operator to v1.31.0
* [CHANGE] Bump k8ssandra-client to v0.8.13, medusa to v0.29.0 and reaper to v4.2.4
* [BUGFIX] Medusa Configurations aren't replicated to remote contexts