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

* [CHANGE] Update cassandra-medusa to 0.22.0
* [CHANGE] Update cass-operator to v1.22.0
* [FEATURE] [#1310](https://github.com/k8ssandra/k8ssandra-operator/issues/1310) Enhance the MedusaBackupSchedule API to allow scheduling purge tasks
* [BUGFIX] [#1222](https://github.com/k8ssandra/k8ssandra-operator/issues/1222) Consider DC-level config when validating numToken updates in webhook
* [BUGFIX] [#1366](https://github.com/k8ssandra/k8ssandra-operator/issues/1366) Reaper deployment can't be created on OpenShift due to missing RBAC rule
