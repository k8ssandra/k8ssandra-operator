---
title: "Releasing k8ssandra-operator"
linkTitle: "Release process"
no_list: false
weight: 1
description: "How to perform releases of k8ssandra-operator"
---


## Major / Minor Release

These steps apply to either creating a major or a minor release.
They should be performed using the k8ssandra/k8ssandra-operator repo as origin (not a fork).

* Create release branch, e.g., `release/1.0`
    * The branch name has to have a prefix of `release/`
* Update the changelog
    * Commit changes
* Update image tag in `config/deployments/default/kustomization.yaml`
    * The image tag should match the git tag
    * If the git tag is `v1.0.0`, then the image tag should also be `v1.0.0`.
    * Commit changes
* Push commits
* Wait for GitHub Actions (GHA) workflows to complete
* After tests have passed, create the git tag, e.g., `v1.0.0`, 
* Push tag
* Wait for GHA workflows to complete
* Create release in GitHub
    * For the release notes just copy over the changelog
* Update the image tag in `config/deployments/default/kustomization.yaml` to `1.0-latest` and `version` in `charts/k8ssandra-operator/Chart.yaml` to `1.0.1-SNAPSHOT`
    * Commit changes using the following as message: `Prepare next patch release`
* Check out `main` branch
* Do a forward merge, e.g., `git merge release/1.0 -s ours`
* Cherry pick changelog updates and any other changes from release that should go back to `main`
    * Do not cherry pick any of the commits for updating `config/deployments/default/kustomization.yaml` and `charts/k8ssandra-operator/Chart.yaml`
    * The image tag in `main` should stay on `latest`
    * Commit amend the foward ported changes, `git commit -a --amend`
* Update the `version` in `charts/k8ssandra-operator/Chart.yaml` to `1.1.0-SNAPSHOT`
    * Commit changes using the following as message: `Prepare next release`
* Push commits to upstream, `git push origin main release/1.0 --atomic`

## Patch Release

* Checkout the target existing release branch, e.g., `release/1.1`
* Cherry pick the commits containing the changes that should be included in the patch release
* Update the changelog
    * Commit changes
* Update image tag in `config/deployments/default/kustomization.yaml`
    * The image tag should match the git tag
    * If the git tag is `v1.1.2`, then the image tag should also be `v1.1.2`.
    * Update the `version` to `1.1.2` in `charts/k8ssandra-operator/Chart.yaml`
    * Commit changes
* Push commits
* Wait for GitHub Actions (GHA) workflows to complete
* After tests have passed, create the git tag, e.g., `v1.1.2`, 
* Push tag
* Wait for GHA workflows to complete
* Create release in GitHub
    * For the release notes just copy over the changelog
* Update the image tag in `config/deployments/default/kustomization.yaml` to `1.1-latest` and `version` in `charts/k8ssandra-operator/Chart.yaml` to `1.1.3-SNAPSHOT`
    * Commit changes using the following as message: `Prepare next patch release`
* Check out `main` branch
* Do a forward merge, e.g., `git merge release/1.1 -s ours`
* Cherry pick changelog updates and any other changes from release that should go back to `main`
    * Do not cherry pick any of the commits for updating `config/deployments/default/kustomization.yaml` and `charts/k8ssandra-operator/Chart.yaml`
    * The image tag in `main` should stay on `latest`
    * Commit amend the foward ported changes, `git commit -a --amend`
* Push commits to upstream, `git push origin main release/1.1 --atomic`