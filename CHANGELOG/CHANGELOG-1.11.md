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

* [CHANGE] Upgrade to Medusa v0.17.0
* [FEATURE] [#659](https://github.com/thelastpickle/cassandra-medusa/issues/659) Add support for DSE search 
* [ENHANCEMENT] [#1125](https://github.com/k8ssandra/k8ssandra-operator/issues/1125) Support Stargate with DSE and upgrade Stargate to 1.0.77
* [ENHANCEMENT] [#1122](https://github.com/k8ssandra/k8ssandra-operator/issues/1122) Expose backup size in MedusaBackup CRD
* [BUGFIX] [#1145](https://github.com/k8ssandra/k8ssandra-operator/issues/1145) Add missing MutatingWebhook configuration to the Helm chart
