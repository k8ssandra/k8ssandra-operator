#!/usr/bin/env bash
#
# Builds the CRDs and RBAC manifests and copies them to the Helm chart in the appropriate places.
# This script requires the following to be installed and available on your path:
#
#    - kustomize
#    - yq

set -e

mkdir -p build/helm
# Generate the CRDs
kustomize build config/crd > charts/k8ssandra-operator/crds/k8ssandra-operator-crds.yaml
# Generate the role.yaml and clusterrole.yaml files using the RBAC generated manifests
kustomize build config/rbac > build/helm/k8ssandra-operator-rbac.yaml
cat charts/templates/role.tmpl.yaml | tee build/helm/role.yaml
cat build/helm/k8ssandra-operator-rbac.yaml | yq 'select(di == 1).rules' | tee -a build/helm/role.yaml
echo "{{- end }}" >> build/helm/role.yaml
cat charts/templates/clusterrole.tmpl.yaml | tee build/helm/clusterrole.yaml
cat build/helm/k8ssandra-operator-rbac.yaml | yq 'select(di == 1).rules' | tee -a build/helm/clusterrole.yaml
echo "{{- end }}" >> build/helm/clusterrole.yaml
cp build/helm/role.yaml charts/k8ssandra-operator/templates/role.yaml
cp build/helm/clusterrole.yaml charts/k8ssandra-operator/templates/clusterrole.yaml
# Generate the leader election role from the RBAC generated manifests
cat charts/templates/leader-role.tmpl.yaml | tee build/helm/leader-role.yaml
cat build/helm/k8ssandra-operator-rbac.yaml | yq 'select(di == 2).rules' | tee -a build/helm/leader-role.yaml
cp build/helm/leader-role.yaml charts/k8ssandra-operator/templates/leader-role.yaml