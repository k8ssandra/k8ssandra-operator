#!/usr/bin/env bash
#
# Builds the CRDs and RBAC manifests and copies them to the Helm chart in the appropriate places.
# This script requires the following to be installed and available on your path:
#
#    - kustomize
#    - yq

set -e

if [ -z "$KUSTOMIZE" ]; then
    KUSTOMIZE=$(command -v kustomize)
fi

mkdir -p build/helm
# Generate the CRDs
$KUSTOMIZE build config/crd > charts/k8ssandra-operator/crds/k8ssandra-operator-crds.yaml
# Generate the role.yaml and clusterrole.yaml files using the RBAC generated manifests
$KUSTOMIZE build config/rbac > build/helm/k8ssandra-operator-rbac.yaml
cat charts/templates/role.tmpl.yaml | tee build/helm/role.yaml > /dev/null
cat build/helm/k8ssandra-operator-rbac.yaml | yq 'select(di == 1).rules' | tee -a build/helm/role.yaml > /dev/null
echo "{{- end }}" >> build/helm/role.yaml
cat charts/templates/clusterrole.tmpl.yaml | tee build/helm/clusterrole.yaml > /dev/null
cat build/helm/k8ssandra-operator-rbac.yaml | yq 'select(di == 1).rules' | tee -a build/helm/clusterrole.yaml > /dev/null
echo "{{- end }}" >> build/helm/clusterrole.yaml
echo "{{- end }}" >> build/helm/clusterrole.yaml # yeah, we need to do this twice
cp build/helm/role.yaml charts/k8ssandra-operator/templates/role.yaml
cp build/helm/clusterrole.yaml charts/k8ssandra-operator/templates/clusterrole.yaml
# Generate the leader election role from the RBAC generated manifests
cat charts/templates/leader-role.tmpl.yaml | tee build/helm/leader-role.yaml > /dev/null
cat build/helm/k8ssandra-operator-rbac.yaml | yq 'select(di == 2).rules' | tee -a build/helm/leader-role.yaml > /dev/null
cp build/helm/leader-role.yaml charts/k8ssandra-operator/templates/leader-role.yaml