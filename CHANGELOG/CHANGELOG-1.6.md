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

* [CHANGE] [#848](https://github.com/k8ssandra/k8ssandra-operator/issues/848) Perform Helm releases in the release workflow
* [FEATURE] [#815](https://github.com/k8ssandra/k8ssandra-operator/issues/815) Add configuration block to CRDs for new Cassandra metrics agent.
* [ENHANCEMENT] [#859](https://github.com/k8ssandra/k8ssandra-operator/issues/859) Remove current usages of managed-by label
* [BUGFIX] [#854](https://github.com/k8ssandra/k8ssandra-operator/issues/854) Use Patch() to update K8ssandraTask status.
* [ENHANCEMENT] [#773](https://github.com/k8ssandra/k8ssandra-operator/issues/773) Allow for CORS to be enabled on Reaper
