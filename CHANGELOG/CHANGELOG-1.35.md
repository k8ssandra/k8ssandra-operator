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

* [FEATURE] [#1832](https://github.com/k8ssandra/k8ssandra-operator/issues/1832) Add PITR APIs and CRDs
* [FEATURE] [#1833](https://github.com/k8ssandra/k8ssandra-operator/issues/1833) Pass PITR config to Medusa, update pod's annotation on a PITR config change so we force a restart
* [FEATURE] [#1838](https://github.com/k8ssandra/k8ssandra-operator/issues/1838) Add a validating webhook for MedusaRestoreJob that requires exactly one of spec.backup and spec.targetTimestamp
* [FEATURE] [#1835](https://github.com/k8ssandra/k8ssandra-operator/issues/1835) Add the PITR branch to the MedusaRestoreJob reconciler
* [FEATURE] [#1836](https://github.com/k8ssandra/k8ssandra-operator/issues/1836) Add a preStop hook to the medusa container and raise the termination grace period when PITR is enabled, so the commit log spool is uploaded before the pod stops
* [FEATURE] [#1837](https://github.com/k8ssandra/k8ssandra-operator/issues/1837) Set a stuck-replay notice in the status message of a PITR MedusaRestoreJob whose DC is not back in 60 minutes
* [FEATURE] [#1859](https://github.com/k8ssandra/k8ssandra-operator/issues/1859) Enable Medusa's Prometheus metrics endpoint and scrape it with the Vector agent, tagged with `container="medusa"`. New `serviceProperties.metricsPort` and `serviceProperties.metricsTLS` (mutual TLS with the gRPC certificates)
* [ENHANCEMENT] [#1831](https://github.com/k8ssandra/k8ssandra-operator/issues/1831) Add PITR RPCs to the Medusa client
* [ENHANCEMENT] [#1834](https://github.com/k8ssandra/k8ssandra-operator/issues/1834) Add `TerminalError`, split `GetHostMap` and add `GetPitrHostMap` for PITR restores.
* [ENHANCEMENT] [#1883](https://github.com/k8ssandra/k8ssandra-operator/issues/1883) Run Medusa purge and prepare_restore tasks on all Cassandra pods in parallel. Set the `medusa.k8ssandra.io/task-concurrency` annotation to `rack` or `pod` to run on one rack or one pod at a time
* [BUGFIX] [#1872](https://github.com/k8ssandra/k8ssandra-operator/issues/1872) Don't re-run finished MedusaRestoreJobs, which restarted or started the datacenter on every operator restart. User secrets are now refreshed before the restore is marked finished
* [BUGFIX] [#1873](https://github.com/k8ssandra/k8ssandra-operator/issues/1873) Reject MedusaRestoreJob spec changes once the restore has started
* [BUGFIX] [#967](https://github.com/k8ssandra/k8ssandra-operator/issues/967) Fix Medusa gRPC client address formatting for IPv6 pod IPs
* [DOCS] []() Add PITR orchestration design doc
* [TESTING] [#1870](https://github.com/k8ssandra/k8ssandra-operator/issues/1870) Raise the host inotify limits before creating kind clusters in CI, as Ubuntu 26.04 runners don't apply them at boot

