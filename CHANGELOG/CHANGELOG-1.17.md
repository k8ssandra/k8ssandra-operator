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

* [CHANGE] [1313](https://github.com/k8ssandra/k8ssandra-operator/issues/1313) upgrade controller-runtime to 1.17 series, Go to 1.21.
* [BUGFIX] [1317](https://github.com/k8ssandra/k8ssandra-operator/issues/1317) Fix issues with caches in cluster scoped deployments where they were continuing to use a multi-namespace scoped cache and not an informer cache.
* [BUGFIX] [1316](https://github.com/k8ssandra/k8ssandra-operator/issues/1316) Fix interchanged intervals and timeouts in tests.
* [FEATURE] Add support for HCD 1.0
