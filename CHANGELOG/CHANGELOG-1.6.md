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

* [BUGFIX] [#925](https://github.com/k8ssandra/k8ssandra-operator/issues/923) Fix invalid metrics names from MCAC and remove the localhost endpoint override from the new metrics agent

## v1.6.1 - 2023-03-22

* [BUGFIX] [#923](https://github.com/k8ssandra/k8ssandra-operator/issues/923) Upgrade cass-operator to v1.15.0 in the helm chart

## v1.6.0 - 2023-03-10

* [CHANGE] [#907](https://github.com/k8ssandra/k8ssandra-operator/issues/907) Update to cass-operator v1.15.0, remove Vector sidecar, instead use cass-operator's server-system-logger Vector agent and only modify its config
* [CHANGE] [#846](https://github.com/k8ssandra/k8ssandra-operator/issues/846) Remove deprecated CassandraBackup and CassandraRestore APIs
* [CHANGE] [#848](https://github.com/k8ssandra/k8ssandra-operator/issues/848) Perform Helm releases in the release workflow
* [CHANGE] [#887](https://github.com/k8ssandra/k8ssandra-operator/issues/887) Fix CVE-2022-32149.
* [CHANGE] [#891](https://github.com/k8ssandra/k8ssandra-operator/issues/848) Update golang.org/x/net to fix several CVEs.
* [FEATURE] [#826](https://github.com/k8ssandra/k8ssandra-operator/issues/836) Support cass-operator DC name overrides
* [FEATURE] [#815](https://github.com/k8ssandra/k8ssandra-operator/issues/815) Add configuration block to CRDs for new Cassandra metrics agent.
* [FEATURE] [#605](https://github.com/k8ssandra/k8ssandra-operator/issues/598) Create a mutating webhook for the internal secrets provider
* [ENHANCEMENT] [#831](https://github.com/k8ssandra/k8ssandra-operator/issues/831) Add CLUSTER_NAME, DATACENTER_NAME and RACK_NAME environment variables to the vector container
* [ENHANCEMENT] [#859](https://github.com/k8ssandra/k8ssandra-operator/issues/859) Remove current usages of managed-by label
* [BUGFIX] [#854](https://github.com/k8ssandra/k8ssandra-operator/issues/854) Use Patch() to update K8ssandraTask status.
* [BUGFIX] [#906](https://github.com/k8ssandra/k8ssandra-operator/issues/906) Remove sources from Vector configuration that have no sink attached to them (with or without transformers)
