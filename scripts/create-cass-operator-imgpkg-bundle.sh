#!/usr/bin/env bash
set -euo pipefail

OUTPUT_FILE="${1:-images-with-digests.txt}"
CHART_DIR="charts"
VALUES_FILE="$CHART_DIR/values.yaml"
TMP_DIR="$(mktemp -d)"

trap 'rm -rf "$TMP_DIR"' EXIT

command -v helm >/dev/null
command -v yq >/dev/null
command -v docker >/dev/null
docker buildx version >/dev/null

helm dependency update "$CHART_DIR" >/dev/null

IMAGES_FILE="$TMP_DIR/images.txt"

{
  helm template mission-control "$CHART_DIR" \
    --values "$VALUES_FILE" \
    --include-crds \
    | yq -rN '.. | select(tag == "!!map" and has("image")) | .image | select(tag == "!!str" and . != "")' -

  yq -rN '.global.imageConfig.images // {} | to_entries | .[] | .value |
    (([.registry, .repository, .name] | map(select(. != null and . != "")) | join("/")) + ":" + .tag)' \
    "$VALUES_FILE"
} | sort -u > "$IMAGES_FILE"

digest_for() {
  docker buildx imagetools inspect "$1" \
    | awk '$1 == "Digest:" { print $2; found = 1; exit } END { exit !found }'
}

{
  echo "# Images with SHA256 digests"
  echo "# Format: image:tag@sha256:digest"
  echo ""

  while IFS= read -r image; do
    digest="$(digest_for "$image")"
    echo "$image@$digest"
  done < "$IMAGES_FILE"

  echo ""
  echo "# Chart Dependencies"
  echo "# =================="
  yq -rN '.dependencies[] | "# " + .name + " (" + .version + ") - " + .repository' \
    "$CHART_DIR/Chart.yaml"
} > "$OUTPUT_FILE"

helm package "$CHART_DIR" --destination "$CHART_DIR" >/dev/null