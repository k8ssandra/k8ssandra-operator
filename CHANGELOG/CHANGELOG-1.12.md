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

## v1.12.0 - 2024-02-02

* [CHANGE] Upgrade to Medusa v0.17.2
* [CHANGE] [#1158](https://github.com/k8ssandra/k8ssandra-operator/issues/1158) Use the MedusaConfiguration API when creating Medusa configuration
* [CHANGE] [#1050](https://github.com/k8ssandra/k8ssandra-operator/issues/1050) Remove unnecessary requeues in the Medusa controllers
* [CHANGE] [#1165](https://github.com/k8ssandra/k8ssandra-operator/issues/1165) Upgrade to Medusa v0.17.1
* [FEATURE] [#1157](https://github.com/k8ssandra/k8ssandra-operator/issues/1157) Add the MedusaConfiguration API
* [FEATURE] [#1165](https://github.com/k8ssandra/k8ssandra-operator/issues/1165) Expose Medusa ssl_verify option to allow disabling cert verification for some on prem S3 compatible systems
* [FEATURE] [#1161](https://github.com/k8ssandra/k8ssandra-operator/issues/1161) Add truststore and keystore properties to the http management api configuration for Reaper
* [FEATURE] [#1152](https://github.com/k8ssandra/k8ssandra-operator/issues/1152) Support AWS IAM roles for Medusa authentication
* [ENHANCEMENT] [#1094](https://github.com/k8ssandra/k8ssandra-operator/issues/1094) Expose AdditionalAnnotations field for cassDC.
* [ENHANCEMENT] [#1160](https://github.com/k8ssandra/k8ssandra-operator/issues/1160) Allow disabling Reaper front-end auth.
* [ENHANCEMENT] [#1115](https://github.com/k8ssandra/k8ssandra-operator/issues/1115) Add a validation check for the projected pod names length
* [ENHANCEMENT] [#1161](https://github.com/k8ssandra/k8ssandra-operator/issues/1161) Update cass-operator Helm chart to 0.46.1. Adds containerPort for cass-operator metrics and changes cass-config-builder base from UBI7 to UBI8
* [ENHANCEMENT] [#1154](https://github.com/k8ssandra/k8ssandra-operator/issues/1154) Schedule purges on clusters that have Medusa configured
* [BUGFIX] [#1002](https://github.com/k8ssandra/k8ssandra-operator/issues/1002) Fix reaper secret name sanitization with cluster overrides
* [BUGFIX] [#1188](https://github.com/k8ssandra/k8ssandra-operator/issues/1188) Fix wrong keyspace replication factor alteration by the operator when multiple dcs with the same name exist
* [BUGFIX] [#1195](https://github.com/k8ssandra/k8ssandra-operator/issues/1195) Backup purge cronjobs use a hardcoded service account name