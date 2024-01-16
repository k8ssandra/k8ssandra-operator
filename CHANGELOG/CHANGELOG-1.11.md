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

## v1.11.1 - 2024-01-16

* [ENHANCEMENT] [#1161](https://github.com/k8ssandra/k8ssandra-operator/issues/1161) Update cass-operator Helm chart to 0.46.1. Adds containerPort for cass-operator metrics and changes cass-config-builder base from UBI7 to UBI8
* [BUGFIX] [#1172](https://github.com/k8ssandra/k8ssandra-operator/issues/1172) Restrict the mutating webhook to cass-operator managed pods

## v1.11.0 - 2023-12-20

* [CHANGE] Upgrade to Medusa v0.17.0
* [CHANGE] Upgrade to cass-operator v1.18.2
* [FEATURE] [#659](https://github.com/thelastpickle/cassandra-medusa/issues/659) Add support for DSE search 
* [ENHANCEMENT] [#1125](https://github.com/k8ssandra/k8ssandra-operator/issues/1125) Support Stargate with DSE and upgrade Stargate to 1.0.77
* [ENHANCEMENT] [#1122](https://github.com/k8ssandra/k8ssandra-operator/issues/1122) Expose backup size in MedusaBackup CRD
* [BUGFIX] [#1145](https://github.com/k8ssandra/k8ssandra-operator/issues/1145) Add missing MutatingWebhook configuration to the Helm chart
* [BUGFIX] [#1132](https://github.com/k8ssandra/k8ssandra-operator/issues/1132) Fix mismatch between Cassandra pods from different cluster when running backup ops
* [BUGFIX] [#1119](https://github.com/k8ssandra/k8ssandra-operator/issues/1119) Fix k8ssandra cluster upgrade failures between 3.x and 4.x versions
