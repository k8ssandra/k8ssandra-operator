#!/usr/bin/env bash
set -euo pipefail

# Generates a Kustomize Component that patches cass-operator's image-config ConfigMap
# by embedding ImageConfig fragments from config/cass-operator/imageconfig/fragments/*
#
# We need to do this because Kustomize does not support patching ConfigMap's keys or
# is able to patch the ImageConfig directly. 
#
# Also, update the kustomization versions based on the Makefile's cass-operator version

if [ -z $CASS_OPERATOR_REF ]; then
  echo "CASS_OPERATOR_REF is not set. "
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
CLUSTER_KUSTOMIZE="${ROOT_DIR}/config/cass-operator/cluster-scoped/kustomization.yaml"
NAMESPACE_KUSTOMIZE="${ROOT_DIR}/config/cass-operator/ns-scoped/kustomization.yaml"

IMAGECONFIG_DIR="${ROOT_DIR}/config/cass-operator/imageconfig"
FRAG_DIR="${IMAGECONFIG_DIR}/fragments"
PATCH_FILE="${IMAGECONFIG_DIR}/patch-image-config.yaml"
COMPONENT_KUSTOMIZE_FILE="${IMAGECONFIG_DIR}/kustomization.yaml"

# Note, while the imageconfig does have kustomization.yaml, we don't want that version as base
# kustomize is unable to do anything to it, so we fetch only the image_config.yaml

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "${TMP_DIR}"; }
trap cleanup EXIT

BASE_URL="https://raw.githubusercontent.com/k8ssandra/cass-operator/${CASS_OPERATOR_REF}/config/imageconfig/image_config.yaml"

curl -fsSLO "${BASE_URL}" --output-dir "${TMP_DIR}"
if [ $? -ne 0 ]; then
    echo "Failed to download base ImageConfig from cass-operator at ${BASE_URL}" >&2
    exit 1
fi

# Sort to ensure we don't unnecessarily update the output for git
FRAG_FILES=()
if [[ -d "${FRAG_DIR}" ]]; then
  while IFS= read -r -d '' f; do
    FRAG_FILES+=("${f}")
  done < <(find "${FRAG_DIR}" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0 | sort -z)
fi

yq -I4 -i ea '. as $item ireduce ({}; . *+ $item)' "${BASE_FILE}" "${FRAG_FILES[@]:-}"

# Lets write the output
mkdir -p "${IMAGECONFIG_DIR}"
cat > "${PATCH_FILE}" << 'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: image-config
data:
  image_config.yaml: |
EOF
cat "${TMP_DIR}/image_config.yaml" >> "${PATCH_FILE}"

cat > "${COMPONENT_KUSTOMIZE_FILE}" <<'EOF'
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component

patches:
  - path: patch-image-config.yaml
    target:
      kind: ConfigMap
      name: image-config
EOF

# We want to maintain the cass-operator version in a single place, so updating Makefile should update the Kustomize templates also.
# I think we want to recommend Helm as primary installation method and only use Kustomize for development purposes
update_ref_in_file() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    return 1
  fi
  sed -i -E "s|(github.com/k8ssandra/cass-operator/[^[:space:]]*\?ref=)[^[:space:]]+|\\1${CASS_OPERATOR_REF}|g" "$file"
}
update_ref_in_file "${CLUSTER_KUSTOMIZE}"
update_ref_in_file "${NAMESPACE_KUSTOMIZE}"
