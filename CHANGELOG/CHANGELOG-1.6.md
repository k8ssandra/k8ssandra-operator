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

* [CHANGE] [#846](https://github.com/k8ssandra/k8ssandra-operator/issues/846) Remove deprecated CassandraBackup and CassandraRestore APIs
* [CHANGE] [#848](https://github.com/k8ssandra/k8ssandra-operator/issues/848) Perform Helm releases in the release workflow
* [FEATURE] [#826](https://github.com/k8ssandra/k8ssandra-operator/issues/836) Support cass-operator DC name overrides
* [FEATURE] [#815](https://github.com/k8ssandra/k8ssandra-operator/issues/815) Add configuration block to CRDs for new Cassandra metrics agent.
* [ENHANCEMENT] [#831](https://github.com/k8ssandra/k8ssandra-operator/issues/831) Add CLUSTER_NAME, DATACENTER_NAME and RACK_NAME environment variables to the vector container
* [ENHANCEMENT] [#859](https://github.com/k8ssandra/k8ssandra-operator/issues/859) Remove current usages of managed-by label
* [BUGFIX] [#854](https://github.com/k8ssandra/k8ssandra-operator/issues/854) Use Patch() to update K8ssandraTask status.
* [CHANGE] [#887](https://github.com/k8ssandra/k8ssandra-operator/issues/887) Fix CVE-2022-32149.
* [CHANGE] [#891](https://github.com/k8ssandra/k8ssandra-operator/issues/848) Update golang.org/x/net to fix several CVEs.
