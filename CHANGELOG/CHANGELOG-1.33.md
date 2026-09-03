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

* [CHANGE] Update cass-operator to v1.32.0
* [CHANGE] Bump k8ssandra-client to v0.8.13, medusa to v0.29.0 and reaper to v4.2.4
* [CHANGE] Bump cassandra-medusa to v0.30.1
* [CHANGE] Bump cassandra-reaper to v5.0.1
* [ENHANCEMENT] [#1760](https://github.com/k8ssandra/k8ssandra-operator/issues/1760) Allow setting Medusa's chunk size
* [BUGFIX] [#1773](https://github.com/k8ssandra/k8ssandra-operator/issues/1773) Medusa Configurations aren't replicated to remote contexts
* [BUGFIX] [#1415](https://github.com/k8ssandra/k8ssandra-operator/issues/1415) Trigger k8ssandrcluster reconcile on referenced MedusaConfiguration update
* [BUGFIX] [#1771](https://github.com/k8ssandra/k8ssandra-operator/issues/1771) ContactPoints services was created by polling the potential pods using dc name only, without namespace filtering
* [BUGFIX] [#1778](https://github.com/k8ssandra/k8ssandra-operator/issues/1778) Add ServerVersion check for Create also in the webhook to prevent invalid K8ssandraCluster creation