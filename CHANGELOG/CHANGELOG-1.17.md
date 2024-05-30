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

* [CHANGE] []() Update cass-operator to v1.21.0
* [CHANGE] [#1313](https://github.com/k8ssandra/k8ssandra-operator/issues/1313) upgrade controller-runtime to 1.17 series, Go to 1.21.
* [BUGFIX] [#1317](https://github.com/k8ssandra/k8ssandra-operator/issues/1317) Fix issues with caches in cluster scoped deployments where they were continuing to use a multi-namespace scoped cache and not an informer cache.
* [BUGFIX] [#1316](https://github.com/k8ssandra/k8ssandra-operator/issues/1316) Fix interchanged intervals and timeouts in tests.
* [BUGFIX] [#1322](https://github.com/k8ssandra/k8ssandra-operator/issues/1322) Fix bug where server-system-logger customisations from the Containers field would be overwritten when vector was enabled. 
* [FEATURE] Add support for HCD 1.0
* [ENHANCEMENT] [#1329](https://github.com/k8ssandra/k8ssandra-operator/issues/1329) Add config emptyDir volume mount on Reaper deployment to allow read only root FS