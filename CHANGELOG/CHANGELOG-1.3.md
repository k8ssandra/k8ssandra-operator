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


## unreleased

## v1.3.0 - 2022-10-12

* [FEATURE] [#657](https://github.com/k8ssandra/k8ssandra-operator/issues/657) Basic DSE Support
* [FEATURE] [#661](https://github.com/k8ssandra/k8ssandra-operator/issues/661) Support all dse.yaml options
* [FEATURE] [#710](https://github.com/k8ssandra/k8ssandra-operator/issues/710) Implement unstructured cassandra and dse yaml configuration
* [ENHANCEMENT] [#681](https://github.com/k8ssandra/k8ssandra-operator/issues/681) Update to Go 1.18, Kubernetes 1.24, Operator SDK 1.23, controller-gen 0.9.2, Kustomize 4.5.7, controller-runtime 0.12.2
* [ENHANCEMENT] [#699](https://github.com/k8ssandra/k8ssandra-operator/issues/699) Prevent DC decommission if user keyspaces are replicated to it
* [ENHANCEMENT] [#695](https://github.com/k8ssandra/k8ssandra-operator/issues/695) Support DSE multi DC clusters
* [ENHANCEMENT] [#669](https://github.com/k8ssandra/k8ssandra-operator/issues/669) Deterministic DSE upgrades
* [ENHANCEMENT] [#704](https://github.com/k8ssandra/k8ssandra-operator/issues/704) Upgrade to cass-operator v1.13.0
* [BUGFIX] [#648](https://github.com/k8ssandra/k8ssandra-operator/issues/648) Open gRPC port within the stargate deployment
* [BUGFIX] [#696](https://github.com/k8ssandra/k8ssandra-operator/issues/696) Upgrade datacenters sequentially instead of concurrently
* [BUGFIX] [#641](https://github.com/k8ssandra/k8ssandra-operator/issues/641) Reaper ServiceMonitor is not properly configured
* [TESTING] [#687](https://github.com/k8ssandra/k8ssandra-operator/issues/687) Verify that modifications to dse.yaml are applied
* [TESTING] Switch to NGINX for ingress in e2e tests
