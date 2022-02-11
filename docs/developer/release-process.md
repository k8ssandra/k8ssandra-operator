# Major / Minor Release
These steps apply to either creating a major or a minor release.

* Create release branch, e.g., `release/1.0`
    * The branch name has to have a prefix of `release/`
* Update the changelog
    * Commit changes
* Update image tag in `config/deployments/default/kustomization.yaml`
    * The image tag should match the git tag
    * If the git tag is `v1.0.0`, then the image tag should also be `v1.0.0`.
    * Commit changes
* Push commits upstream
* Wait for GitHub Actions (GHA) workflows to complete
* After tests have passed, create the git tag, e.g., `v1.0.0`, 
* Push tag upstream
* Wait for GHA workflow to complete
* Create release in GitHub
    * For the release notes just copy over the changelog
* Check out `main` branch
* Do a forward merge, e.g., `git merge release/1.0 -s ours`
* Cherry pick changelog updates and any other changes from release that should go back to `main`
    * Do not cherry pick the commit for updating `config/deployments/default/kustomization.yaml`
    * The image tag in `main` should stay on `latest`
    * Commit amend the foward ported changes, `git commit -a --amend`
    * Push commits to upstream, `git push upstream main`
* Update Helm chart (see section below)  

# Update k8ssandra-operator Helm Chart
When we release a new version of the operator, we need to also update the Helm chart. It lives in the k8ssandra repo [here](https://github.com/k8ssandra/k8ssandra/tree/main/charts/k8ssandra-operator).

Create a PR for the k8ssandra repo that includes the following changes:

* Generate CRD manifests by running `kustomize build config/crd > /tmp/k8ssandra-operator-crds.yaml` in k8ssandra-operator repo
    * Make sure to do this from the release tag
* Empty the `charts/k8ssandra-operator/crds` directory in k8ssandra repo and move `/tmp/k8ssandra-operator-crds.yaml` in it.
* Generate RBAC manifests by running `kustomize build config/rbac` in k8ssandra-operator repo
    * Make sure to do this from the release tag
* Apply RBAC changes in `charts/k8ssandra-operator/templates` in k8ssandra repo
* Update the `image.tag` property in `charts/k8ssandra-operator/values.yaml`, e.g., `v1.0.0`
* Update the `version` and `appVersion` properties in `charts/k8ssandra-operator/Chart.yaml`

When the PR is merged, GHA will upload the new chart into the k8ssandra chart repo. 