---
title: "Releasing k8ssandra-operator"
linkTitle: "Release process"
no_list: false
weight: 11
description: "How to perform releases of k8ssandra-operator"
---

## Prerequisites

- [yq](https://github.com/mikefarah/yq) installed.
- [Hugo](https://gohugo.io/documentation/) installed.

## Major / Minor Release

### Preliminary steps

Create these environment variables, we'll reference them throughout the rest of this section:

- the Major.minor coordinates of the version we are about to release:
    ```shell
    export RELEASED_MAJOR_MINOR=1.6
    ```
- the Major.minor coordinates of the next version on the main branch (usually this will be the next minor, but there
  could also be a major bump, like 2.0):
    ```shell
    export NEXT_MAJOR_MINOR=1.7
    ```
- the name of your Git remote for k8ssandra/k8ssandra-operator:
    ```shell
    export GIT_REMOTE=origin
    ```

In your local `k8ssandra-operator` clone, check out the main branch and make sure it's up-to-date:
```shell
git checkout main
git pull ${GIT_REMOTE?} main
```

### Create the release branch

```shell
git checkout -b release/${RELEASED_MAJOR_MINOR?}
```

### Update the changelog

Edit `CHANGELOG/CHANGELOG-${RELEASED_MAJOR_MINOR?}.md` and replace the `## unreleased` section with the new release
version and date (keep the `## unreleased` section above it).

The diff should look like this:
```diff
@@ -15,6 +15,8 @@ When cutting a new release, update the `unreleased` heading to the tag being gen

 ## unreleased

+## v1.6.0 - 2023-03-10
+
 * [CHANGE] [#907](https://github.com/k8ssandra/k8ssandra-operator/issues/907) Update to cass-operator v1.15.0, remove Vector sidecar, instead use cass-operator's server-system-logger Vector agent and only modify its config
```

Commit the changes:
```shell
git add CHANGELOG/CHANGELOG-${RELEASED_MAJOR_MINOR?}.md
git commit -m"Update changelog for v${RELEASED_MAJOR_MINOR?}.0"
```

### Check that the documentation builds

```shell
cd docs
hugo mod clean
npm run build:production

cd ..
```

If there are any errors, fix them and commit them on the release branch.

### Create the release commit

Update the image tag in the default kustomization:
```shell
yq e -i '.images[0].newTag="v"+env(RELEASED_MAJOR_MINOR)+".0"' config/deployments/default/kustomization.yaml
```

The diff should look like this:
```diff
@@ -8,5 +8,4 @@ resources:

 images:
   - name: k8ssandra/k8ssandra-operator
-    newTag: latest
-
+    newTag: v1.6.0
```

Update the version in the release chart:
```shell
yq e -i '.version=env(RELEASED_MAJOR_MINOR)+".0"' charts/k8ssandra-operator/Chart.yaml
```

The diff should look like this:
```diff
@@ -3,7 +3,7 @@ name: k8ssandra-operator
 description: |
   Kubernetes operator which handles the provisioning and management of K8ssandra clusters.
 type: application
-version: 1.6.0-SNAPSHOT
+version: 1.6.0
 dependencies:
   - name: k8ssandra-common
     version: 0.29.0
```

Commit and push the changes:
```shell
git add config/deployments/default/kustomization.yaml charts/k8ssandra-operator/Chart.yaml
git commit -m"Release v${RELEASED_MAJOR_MINOR?}.0"
git push ${GIT_REMOTE?} release/${RELEASED_MAJOR_MINOR?}
```

Wait for the [GitHub Actions](https://github.com/k8ssandra/k8ssandra-operator/actions) workflows to complete.

### Tag the release

```shell
git tag v${RELEASED_MAJOR_MINOR?}.0
git push ${GIT_REMOTE?} v${RELEASED_MAJOR_MINOR?}.0
```

Wait for the [GitHub Actions](https://github.com/k8ssandra/k8ssandra-operator/actions) workflows to complete.

### Create the release in GitHub

Go to the [New Release](https://github.com/k8ssandra/k8ssandra-operator/releases/new) page.

- click on "Choose a tag" and select the tag you just pushed, e.g. `v1.6.0`.
- for the release title, use the name of the tag, e.g. "v1.6.0".
- for the description, just copy the contents for this version from the changelog.

Click on "Publish release".

### Prepare the branch for the next patch release

Update the image tag in the default kustomization:
```shell
yq e -i '.images[0].newTag=env(RELEASED_MAJOR_MINOR)+"-latest"' config/deployments/default/kustomization.yaml
```

The diff should look like this:
```diff
@@ -8,5 +8,4 @@ resources:

 images:
   - name: k8ssandra/k8ssandra-operator
-    newTag: v1.6.0
-
+    newTag: 1.6-latest
```

Update the version in the release chart:
```shell
yq e -i '.version=env(RELEASED_MAJOR_MINOR)+".1-SNAPSHOT"' charts/k8ssandra-operator/Chart.yaml
```

The diff should look like this:
```diff
@@ -3,7 +3,7 @@ name: k8ssandra-operator
 description: |
   Kubernetes operator which handles the provisioning and management of K8ssandra clusters.
 type: application
-version: 1.6.0
+version: 1.6.1-SNAPSHOT
 dependencies:
   - name: k8ssandra-common
     version: 0.29.0
```

Commit the changes:
```shell
git add config/deployments/default/kustomization.yaml charts/k8ssandra-operator/Chart.yaml
git commit -m"Prepare next patch release"
```

### Update the main branch

Merge with the "ours" strategy (none of the changes are applied):
```shell
git checkout main
git merge release/${RELEASED_MAJOR_MINOR?} -s ours
```

Then cherry-pick the changelog update, as well as any fixes you might have done on the branch (docs, failing tests, etc):
```shell
git cherry-pick -n <your commit SHA-1s...>
```

**DO NOT** cherry-pick the "Release vx.x.x" or "Prepare next patch release" commits.

Amend the merge commit with your cherry-picks:
```shell
git add .
git commit --amend
```

Update the version in the release chart:
```shell
yq e -i '.version=env(NEXT_MAJOR_MINOR)+".0-SNAPSHOT"' charts/k8ssandra-operator/Chart.yaml
```

The diff should look like this:
```diff
@@ -3,7 +3,7 @@ name: k8ssandra-operator
 description: |
   Kubernetes operator which handles the provisioning and management of K8ssandra clusters.
 type: application
-version: 1.6.0-SNAPSHOT
+version: 1.7.0-SNAPSHOT
 dependencies:
   - name: k8ssandra-common
     version: 0.29.0
```

Create a new blank changelog file `CHANGELOG/CHANGELOG-${NEXT_MAJOR_MINOR?}.md` with the following contents:

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

Commit the changes:
```shell
git add charts/k8ssandra-operator/Chart.yaml CHANGELOG/CHANGELOG-${NEXT_MAJOR_MINOR?}.md
git commit -m"Prepare next release"
```

Push all the changes:
```shell
git push ${GIT_REMOTE?} main release/${RELEASED_MAJOR_MINOR?} --atomic
```


## Patch Release

### Preliminary steps

Create these environment variables, we'll reference them throughout the rest of this section:

- the Major.minor coordinates of the version we are about to release:
    ```shell
    export RELEASED_MAJOR_MINOR=1.6
    ```
- the full coordinates of the version we are about to release:
    ```shell
    export RELEASED_FULL=1.6.1
    ```
- the full coordinates of the next version on the release branch:
    ```shell
    export NEXT_FULL=1.6.2
    ```
- the name of your Git remote for k8ssandra/k8ssandra-operator:
    ```shell
    export GIT_REMOTE=origin
    ```

In your local `k8ssandra-operator` clone, make sure the main and release branches are up-to-date, and switch to the
release branch:
```shell
git checkout main
git pull ${GIT_REMOTE?} main

git checkout release/${RELEASED_MAJOR_MINOR?}
git pull ${GIT_REMOTE?} release/${RELEASED_MAJOR_MINOR?}
```

### Cherry-pick the changes from main

Select the commits to include from the `main` branch:
```shell
git cherry-pick <SHA1...> 
```

### Update the changelog

Edit `CHANGELOG/CHANGELOG-${RELEASED_MAJOR_MINOR?}.md` and replace the `## unreleased` section with the new release
version and date (keep the `## unreleased` section above it).

The diff should look like this:
```diff
@@ -15,6 +15,8 @@ When cutting a new release, update the `unreleased` heading to the tag being gen

 ## unreleased

+## v1.6.1 - 2023-04-10
+
 * [CHANGE] [#907](https://github.com/k8ssandra/k8ssandra-operator/issues/907) Update to cass-operator v1.15.0, remove Vector sidecar, instead use cass-operator's server-system-logger Vector agent and only modify its config
```

Commit the changes:
```shell
git add CHANGELOG/CHANGELOG-${RELEASED_MAJOR_MINOR?}.md
git commit -m"Update changelog for v${RELEASED_FULL?}"
```

### Check that the documentation builds

```shell
cd docs
hugo mod clean
npm run build:production

cd ..
```

If there are any errors, fix them and commit them on the release branch.

### Create the release commit

Update the image tag in the default kustomization:
```shell
yq e -i '.images[0].newTag="v"+env(RELEASED_FULL)' config/deployments/default/kustomization.yaml
```

The diff should look like this:
```diff
@@ -8,5 +8,4 @@ resources:

 images:
   - name: k8ssandra/k8ssandra-operator
-    newTag: 1.6-latest
-
+    newTag: v1.6.1
```

Update the version in the release chart:
```shell
yq e -i '.version=env(RELEASED_FULL)' charts/k8ssandra-operator/Chart.yaml
```

The diff should look like this:
```diff
@@ -3,7 +3,7 @@ name: k8ssandra-operator
 description: |
   Kubernetes operator which handles the provisioning and management of K8ssandra clusters.
 type: application
-version: 1.6.1-SNAPSHOT
+version: 1.6.1
 dependencies:
   - name: k8ssandra-common
     version: 0.29.0
```

Commit and push the changes:
```shell
git add config/deployments/default/kustomization.yaml charts/k8ssandra-operator/Chart.yaml
git commit -m"Release v${RELEASED_FULL?}"
git push ${GIT_REMOTE?} release/${RELEASED_MAJOR_MINOR?}
```

Wait for the [GitHub Actions](https://github.com/k8ssandra/k8ssandra-operator/actions) workflows to complete.

### Tag the release

```shell
git tag v${RELEASED_FULL?}
git push ${GIT_REMOTE?} v${RELEASED_FULL?}
```

Wait for the [GitHub Actions](https://github.com/k8ssandra/k8ssandra-operator/actions) workflows to complete.

### Create the release in GitHub

Go to the [New Release](https://github.com/k8ssandra/k8ssandra-operator/releases/new) page.

- click on "Choose a tag" and select the tag you just pushed, e.g. `v1.6.1`.
- for the release title, use the name of the tag, e.g. "v1.6.1".
- for the description, just copy the contents for this version from the changelog.

Click on "Publish release".

### Prepare the branch for the next patch release

Update the image tag in the default kustomization:
```shell
yq e -i '.images[0].newTag=env(RELEASED_MAJOR_MINOR)+"-latest"' config/deployments/default/kustomization.yaml
```

The diff should look like this:
```diff
@@ -8,5 +8,4 @@ resources:

 images:
   - name: k8ssandra/k8ssandra-operator
-    newTag: v1.6.1
-
+    newTag: 1.6-latest
```

Update the version in the release chart:
```shell
yq e -i '.version=env(NEXT_FULL)+"-SNAPSHOT"' charts/k8ssandra-operator/Chart.yaml
```

The diff should look like this:
```diff
@@ -3,7 +3,7 @@ name: k8ssandra-operator
 description: |
   Kubernetes operator which handles the provisioning and management of K8ssandra clusters.
 type: application
-version: 1.6.1
+version: 1.6.2-SNAPSHOT
 dependencies:
   - name: k8ssandra-common
     version: 0.29.0
```

Commit the changes:
```shell
git add config/deployments/default/kustomization.yaml charts/k8ssandra-operator/Chart.yaml
git commit -m"Prepare next patch release"
```

### Update the main branch

Merge with the "ours" strategy (none of the changes are applied):
```shell
git checkout main
git merge release/${RELEASED_MAJOR_MINOR?} -s ours
```

Then cherry-pick the changelog update, as well as any fixes you might have done on the branch (docs, failing tests, etc):
```shell
git cherry-pick -n <your commit SHA-1s...>
```

**DO NOT** cherry-pick the release contents that you cherry-picked in a previous step (they are already on `main`).
**DO NOT** cherry-pick the "Release vx.x.x" or "Prepare next patch release" commits.

Amend the merge commit with your cherry-picks:
```shell
git add .
git commit --amend
```

Push all the changes:
```shell
git push ${GIT_REMOTE?} main release/${RELEASED_MAJOR_MINOR?} --atomic
```
