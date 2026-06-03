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

## v1.32.2 - 2026-06-03

* [CHANGE] Bump cass-operator to v1.30.2 (0.64.3) and k8ssandra-client to v0.8.13
* [CHANGE] Bump Reaper to v4.2.4
* [CHANGE] Bump Medusa to 0.29.0

## v1.32.1 - 2026-05-20

* [CHANGE] Bump Reaper to v4.2.3

## v1.32.0 - 2026-04-29

* [CHANGE] [#1731](https://github.com/k8ssandra/k8ssandra-operator/issues/1713) Bump Medusa to 0.28.0
* [CHANGE] [1706](https://github.com/k8ssandra/k8ssandra-operator/issues/1706) Bump Reaper to 4.2.1 (with sqlite-backed local storage)
* [CHANGE] [#1720](https://github.com/k8ssandra/k8ssandra-operator/issues/1720) Do not retain Reaper's PVC once if Reaper uses local storage and gets deleted
* [CHANGE] [#1725](https://github.com/k8ssandra/k8ssandra-operator/issues/1725) Bump cass-operator helm chart to 0.64.1
* [BUGFIX] [#1717](https://github.com/k8ssandra/k8ssandra-operator/issues/1717) When creating Telemetry config for Vector, take into account Datacenter level as well as Cluster level settings
