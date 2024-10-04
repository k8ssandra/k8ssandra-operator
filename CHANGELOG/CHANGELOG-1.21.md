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

* [CHANGE] Bump default Medusa version to 0.22.3
* [BUGFIX] [#1409](https://github.com/k8ssandra/k8ssandra-operator/issues/1409) Vector would crash in the Cassandra log parsing if empty lines were present. Add automated tests for Vector parsing rules.
* [BUGFIX] [#1425](https://github.com/k8ssandra/k8ssandra-operator/issues/1425) prepare-helm-release.sh requires kustomize to be in the path and that makes make manifests fail. 
* [DOCS] [#1469](https://github.com/riptano/mission-control/issues/1469) Add docs for Reaper's Control Plane deployment mode
