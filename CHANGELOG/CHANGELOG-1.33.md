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

* [ENHANCEMENT] [#952](https://github.com/k8ssandra/k8ssandra-operator/issues/952) Set an owner reference on the CassandraDatacenter pointing to the K8ssandraCluster when they are co-located (same namespace and cluster). This improves visibility in tools like ArgoCD. Cross-namespace/cluster deployments are unaffected and keep relying on labels and finalizers.
* [CHANGE] Update cass-operator to v1.31.0
* [CHANGE] Bump k8ssandra-client to v0.8.13, medusa to v0.29.0 and reaper to v4.2.4
