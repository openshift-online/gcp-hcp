#!/bin/bash
# Update external-secrets manifests from upstream Helm chart
#
# Usage: ./update.sh [VERSION]
#   VERSION: Chart version (e.g., 0.17.0). If not specified, shows current
#            and latest versions without making changes.
#
# This script generates controller.yaml using helm template with the correct
# namespace and replica settings for our deployment.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION="${1:-}"
NAMESPACE="external-secrets-system"
CHART="oci://ghcr.io/external-secrets/charts/external-secrets"

# Get current version from controller.yaml (extract from image tag)
get_current_version() {
    if [[ -f "$SCRIPT_DIR/controller.yaml" ]]; then
        grep -m1 'image:.*external-secrets:' "$SCRIPT_DIR/controller.yaml" 2>/dev/null | \
            sed 's/.*external-secrets://' | tr -d ' "' || echo "unknown"
    else
        echo "not installed"
    fi
}

# Get latest chart version
get_latest_version() {
    helm show chart "$CHART" 2>/dev/null | grep '^version:' | awk '{print $2}'
}

CURRENT_VERSION=$(get_current_version)
LATEST_VERSION=$(get_latest_version)

# If no version specified, show current vs latest and exit
if [[ -z "$VERSION" ]]; then
    echo "external-secrets versions:"
    echo "  Current: $CURRENT_VERSION"
    echo "  Latest:  $LATEST_VERSION"
    echo ""
    if [[ "$CURRENT_VERSION" == "$LATEST_VERSION" ]]; then
        echo "Already up to date."
    else
        echo "To update, run: ./update.sh $LATEST_VERSION"
    fi
    exit 0
fi

echo "Updating external-secrets from $CURRENT_VERSION to $VERSION..."

# Generate controller.yaml with helm template
helm template external-secrets "$CHART" \
    --version "$VERSION" \
    --namespace "$NAMESPACE" \
    --set certController.replicaCount=2 \
    --set webhook.replicaCount=2 \
    > "$SCRIPT_DIR/controller.yaml"

echo "Generated controller.yaml from external-secrets chart version $VERSION"
echo ""
echo "Next steps:"
echo "  1. Review changes: git diff $SCRIPT_DIR/controller.yaml"
echo "  2. Commit: git add $SCRIPT_DIR && git commit -m 'chore(external-secrets): update to v$VERSION'"
echo "  3. Push to trigger Config Sync"
