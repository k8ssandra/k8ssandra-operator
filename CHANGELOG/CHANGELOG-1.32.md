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

## unreleased

## v1.32.0 - 2026-04-29

* [CHANGE] [#1731](https://github.com/k8ssandra/k8ssandra-operator/issues/1713) Bump Medusa to 0.28.0
* [CHANGE] [1706](https://github.com/k8ssandra/k8ssandra-operator/issues/1706) Bump Reaper to 4.2.1 (with sqlite-backed local storage)
* [CHANGE] [#1720](https://github.com/k8ssandra/k8ssandra-operator/issues/1720) Do not retain Reaper's PVC once if Reaper uses local storage and gets deleted
* [CHANGE] [#1725](https://github.com/k8ssandra/k8ssandra-operator/issues/1725) Bump cass-operator helm chart to 0.64.1
* [BUGFIX] [#1717](https://github.com/k8ssandra/k8ssandra-operator/issues/1717) When creating Telemetry config for Vector, take into account Datacenter level as well as Cluster level settings
