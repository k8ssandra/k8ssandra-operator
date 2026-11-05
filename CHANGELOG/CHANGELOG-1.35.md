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

When cutting a new release, update the `unreleased` heading to the tag being generated and date, like `## vX.Y.Z - YYYY-MM-DD` and create a new placeholder section for `unreleased` entries.

## unreleased

* [DOCS] []() Add PITR orchestration design doc
* [ENHANCEMENT] [#1831](https://github.com/k8ssandra/k8ssandra-operator/issues/1831) Add PITR RPCs to the Medusa client
* [FEATURE] [#1832](https://github.com/k8ssandra/k8ssandra-operator/issues/1832) Add PITR APIs and CRDs
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Requeue rebuild polling after the default delay instead of 15ns
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Reconcile Reaper and telemetry for the correct DC when datacenters are reordered by upgrade priority
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Fail MedusaTask pods left in progress by an operator restart, so the task and later purges are not blocked
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Run Medusa purge schedules for DCs that use a datacenterName override
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Emit an event when a MedusaBackupSchedule postpones an execution because of unfinished jobs
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Keep K8ssandraCluster status updates made earlier in a reconcile when the operator writes annotations or finalizers mid-pass
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Only mark a MedusaBackupJob finished once its MedusaBackup and status are created, and fail InProgress pods that no longer exist
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Skip K8ssandraCluster webhook validation for metadata-only updates and objects being deleted
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Do not create a second refresh K8ssandraTask when the spec changes during a refresh
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Surface contact-points service reconcile errors and mirror one Endpoint per address
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Report an error when the per-node ConfigMap is not owned by the operator
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Clean up DCs that are still being decommissioned when a K8ssandraCluster is deleted
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Preserve finalizers and annotations set by others when updating a CassandraDatacenter
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Apply a stop requested during a DC rebuild once the rebuild finishes
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Never pick the rebuild target DC as its own source when it uses a datacenterName override
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Allow deleting a K8ssandraTask whose datacenter was removed from the cluster, and run explicit DC lists in listed order
