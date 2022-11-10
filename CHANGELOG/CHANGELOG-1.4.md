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

* [ENHANCEMENT] [#740](https://github.com/k8ssandra/k8ssandra-operator/issues/740)  Expose ManagementApiAuth and pod SecurityGroup from the cassdc spec in k8ssandra-operator
* [ENHANCEMENT] [#737](https://github.com/k8ssandra/k8ssandra-operator/issues/737) Support additional jvm options for the various options files 
* [FEATURE] [#599](https://github.com/k8ssandra/k8ssandra-operator/issues/599) Introduce a secrets provider setting in the CRD
* [FEATURE] [#728](https://github.com/k8ssandra/k8ssandra-operator/issues/728) Add token generation utility
* [FEATURE] [#724](https://github.com/k8ssandra/k8ssandra-operator/issues/724) Ability to provide per-node configuration
* [FEATURE] [#718](https://github.com/k8ssandra/k8ssandra-operator/issues/718) Make keystore-password, keystore, truststore keys in secret configurable
* [BUGFIX] [#744](https://github.com/k8ssandra/k8ssandra-operator/issues/744) Handle auth for DSE clusters
* [BUGFIX] [#722](https://github.com/k8ssandra/k8ssandra-operator/issues/722) Enable client-side CQL encryption in Stargate if it is configured on the cluster
* [BUGFIX] [#714](https://github.com/k8ssandra/k8ssandra-operator/issues/714) Don't restart whole Cassandra 4 DC when Stargate is added or removed
* [BUGFIX] [#758](https://github.com/k8ssandra/k8ssandra-operator/issues/758) Remove -jdk8 suffix for DSE images
* [TESTING] [#749](https://github.com/k8ssandra/k8ssandra-operator/issues/749) Add e2e test for distinct commit log and data volumes
* [TESTING] [#747](https://github.com/k8ssandra/k8ssandra-operator/issues/747) Add e2e test and docs for JBOD support
* [BUGFIX] [#755](https://github.com/k8ssandra/k8ssandra-operator/issues/755) Move jvm11 options to jvm8 for DSE
