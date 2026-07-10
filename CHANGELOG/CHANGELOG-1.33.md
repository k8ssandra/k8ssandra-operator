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

* [ENHANCEMENT] [#1752](https://github.com/k8ssandra/k8ssandra-operator/issues/1752) Bound the informer cache (pkg/leancache): read Secrets/ConfigMaps/Pods/Services/Endpoints/EndpointSlices/Deployments/StatefulSets/CronJobs/ServiceMonitors/MedusaBackups through live API calls, project the corresponding watches to metadata only, and strip managedFields/last-applied from cached objects. Operator memory no longer scales with cluster size, and the operator no longer holds every cluster Secret in process memory.
* [CHANGE] Update cass-operator to v1.31.0
* [CHANGE] Bump k8ssandra-client to v0.8.13, medusa to v0.29.0 and reaper to v4.2.4
