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

* [CHANGE] [#1731](https://github.com/k8ssandra/k8ssandra-operator/issues/1713) Bump Medusa to 0.28.0
* [CHANGE] (#1706)[https://github.com/k8ssandra/k8ssandra-operator/issues/1706] Bump Reaper to 4.2.1 (with sqlite-backed local storage)
* [BUGFIX] [#1717](https://github.com/k8ssandra/k8ssandra-operator/issues/1717) When creating Telemetry config for Vector, take into account Datacenter level as well as Cluster level settings
