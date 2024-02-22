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

* [ENHANCEMENT] [#1159](https://github.com/k8ssandra/k8ssandra-operator/issues/1159) Replicate bucket key secrets to namespaces hosting clusters
* [ENHANCEMENT] [#1203](https://github.com/k8ssandra/k8ssandra-operator/issues/1203) Add new setting ConcurrencyPolicy to MedusaBackupSchedules
* [ENHANCEMENT] [#1209](https://github.com/k8ssandra/k8ssandra-operator/issues/1209) Expose the option to disable the cert-manager presence check in the Helm charts
* [ENHANCEMENT] [#1206](https://github.com/k8ssandra/k8ssandra-operator/issues/1206) Allow setting https proxy for CRD upgrader job in Helm charts
* [CHANGE] Upgrade Reaper to v3.5.0
