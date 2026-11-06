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
* [BUGFIX] [#1871](https://github.com/k8ssandra/k8ssandra-operator/issues/1871) Fix a panic when removing a DC in the control-plane cluster while the first remaining DC is in a remote cluster, or when the removed DC's CassandraDatacenter is already gone
* [TESTING] [#1870](https://github.com/k8ssandra/k8ssandra-operator/issues/1870) Raise the host inotify limits before creating kind clusters in CI, as Ubuntu 26.04 runners don't apply them at boot
