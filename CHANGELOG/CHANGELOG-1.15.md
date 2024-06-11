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

## v1.15.0 - 2024-04-18

* [BUGFIX] [#1266](https://github.com/k8ssandra/k8ssandra-operator/issues/1266) MedusaConfigurations must now be namespace local to the K8ssandraCluster they are attached to, a webhook error will be thrown otherwise (for new clusters only). Additionally, ReplicatedSecrets should only pick up secrets from their local namespace to replicate.
* [BUGFIX] [#1217](https://github.com/k8ssandra/k8ssandra-operator/issues/1217) Medusa storage secrets now use a ReplicatedSecret for synchronization, fixing an issue where changes to the secrets were not propagating. Additionally, fix a number of issues with local testing on ARM Macs.
* [BUGFIX] [#1253](https://github.com/k8ssandra/k8ssandra-operator/issues/1253) Medusa storage secrets are now labelled with a unique label.
* [FEATURE] [#1260](https://github.com/k8ssandra/k8ssandra-operator/issues/1260) Update controller-gen to version 0.14.0.
* [BUGFIX] [1287](https://github.com/k8ssandra/k8ssandra-operator/pull/1287) Use the same image for Reaper init and main containers
* [ENHANCEMENT] [1288](https://github.com/k8ssandra/k8ssandra-operator/issues/1288) Allow disabling the CRD upgrader
* [CHANGE] Upgrade Reaper to v3.6.0
* [FEATURE] [#1280](https://github.com/k8ssandra/k8ssandra-operator/issues/1280) Env variables DEFAULT_REGISTRY and IMAGE_PULL_SECRETS allow overriding the default imagePullSecrets as well as the default registry to use when deploying medusa/reaper/stargate.
* [BUGFIX] [#1240](https://github.com/k8ssandra/k8ssandra-operator/issues/1240) The PullSecretRef for medusa is ignored in the standalone deployment of medusa
