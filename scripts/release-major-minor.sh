#!/bin/bash

# Major/Minor Release Script for k8ssandra-operator
# This script automates the major/minor release process as documented in:
# https://docs.k8ssandra.io/contribute/releasing/#major--minor-release

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to prompt for confirmation
confirm() {
    read -p "$1 (y/n): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_error "Operation cancelled by user"
        exit 1
    fi
}

# Check if we're in the right directory
if [ ! -f "go.mod" ] || [ ! -d "apis" ]; then
    print_error "This script must be run from the k8ssandra-operator root directory"
    exit 1
fi

print_info "=== K8ssandra Operator Major/Minor Release Script ==="
echo

# Preliminary steps - gather environment variables
print_info "Step 1: Setting up environment variables"
echo

read -p "Enter the Major.minor version being released (e.g., 1.6): " RELEASED_MAJOR_MINOR
if [ -z "$RELEASED_MAJOR_MINOR" ]; then
    print_error "Released version cannot be empty"
    exit 1
fi

read -p "Enter the next Major.minor version on main branch (e.g., 1.7): " NEXT_MAJOR_MINOR
if [ -z "$NEXT_MAJOR_MINOR" ]; then
    print_error "Next version cannot be empty"
    exit 1
fi

read -p "Enter your Git remote name for k8ssandra/k8ssandra-operator (default: origin): " GIT_REMOTE
GIT_REMOTE=${GIT_REMOTE:-origin}

export RELEASED_MAJOR_MINOR
export NEXT_MAJOR_MINOR
export GIT_REMOTE

print_info "Configuration:"
echo "  Released version: $RELEASED_MAJOR_MINOR"
echo "  Next version: $NEXT_MAJOR_MINOR"
echo "  Git remote: $GIT_REMOTE"
echo

confirm "Continue with these settings?"

# Ensure we're on main and up-to-date
print_info "Step 2: Checking out and updating main branch"
git checkout main
git pull ${GIT_REMOTE} main

# Create the release branch
print_info "Step 3: Creating release branch"
RELEASE_BRANCH="release/${RELEASED_MAJOR_MINOR}"
git checkout -b ${RELEASE_BRANCH}
print_info "Created and checked out branch: ${RELEASE_BRANCH}"

# Update the changelog
print_info "Step 4: Updating the changelog"
CHANGELOG_FILE="CHANGELOG/CHANGELOG-${RELEASED_MAJOR_MINOR}.md"

if [ ! -f "$CHANGELOG_FILE" ]; then
    print_error "Changelog file not found: $CHANGELOG_FILE"
    exit 1
fi

print_warning "Please edit $CHANGELOG_FILE manually:"
echo "  - Replace the '## unreleased' section with the new release version and date"
echo "  - Keep the '## unreleased' section above it"
echo
confirm "Press 'y' when you have finished editing the changelog"

# Commit the changelog changes
print_info "Committing changelog changes"
git add ${CHANGELOG_FILE}
git commit -m "Update changelog for v${RELEASED_MAJOR_MINOR}.0"

# Check documentation builds
print_info "Step 5: Checking that documentation builds"
cd docs
npm install
npm run clean
npm run build:production
cd ..

print_info "Documentation built successfully"
confirm "Did the documentation build without errors?"

# Create the release commit
print_info "Step 6: Creating the release commit"

# Update image tag in default kustomization
print_info "Updating image tag in config/deployments/default/kustomization.yaml"
yq e -i ".images[0].newTag=\"v${RELEASED_MAJOR_MINOR}.0\"" config/deployments/default/kustomization.yaml

# Update version in release chart
print_info "Updating version in charts/k8ssandra-operator/Chart.yaml"
yq e -i ".version=env(RELEASED_MAJOR_MINOR)+\".0\"" charts/k8ssandra-operator/Chart.yaml

# Show the diffs
print_info "Changes to be committed:"
git diff config/deployments/default/kustomization.yaml charts/k8ssandra-operator/Chart.yaml

confirm "Do these changes look correct?"

# Commit the changes
git add config/deployments/default/kustomization.yaml charts/k8ssandra-operator/Chart.yaml
git commit -m "Release v${RELEASED_MAJOR_MINOR}.0"

# Push the changes
print_info "Step 7: Pushing the release branch"
git push ${GIT_REMOTE} ${RELEASE_BRANCH}

print_info "Waiting for GitHub Actions to complete..."
print_warning "Please check GitHub Actions at: https://github.com/k8ssandra/k8ssandra-operator/actions"
confirm "Have all GitHub Actions workflows completed successfully?"

# Tag the release
print_info "Step 8: Tagging the release"
git tag v${RELEASED_MAJOR_MINOR}.0
git push ${GIT_REMOTE} v${RELEASED_MAJOR_MINOR}.0

print_info "Waiting for GitHub Actions to complete..."
confirm "Have all GitHub Actions workflows completed successfully?"

# Create GitHub release
print_info "Step 9: Creating GitHub release"
print_warning "Please go to: https://github.com/k8ssandra/k8ssandra-operator/releases/new"
echo "  1. Choose tag: v${RELEASED_MAJOR_MINOR}.0"
echo "  2. Set title to: v${RELEASED_MAJOR_MINOR}.0"
echo "  3. Copy the changelog content for this version as the description"
echo "  4. Click 'Publish release'"
echo
confirm "Have you created the GitHub release?"

# Prepare the branch for next patch release
print_info "Step 10: Preparing branch for next patch release"

# Update image tag to -latest
yq e -i ".images[0].newTag=env(RELEASED_MAJOR_MINOR)+\"-latest\"" config/deployments/default/kustomization.yaml

# Update version in chart to SNAPSHOT
yq e -i ".version=env(RELEASED_MAJOR_MINOR)+\".1-SNAPSHOT\"" charts/k8ssandra-operator/Chart.yaml

# Show the diffs
print_info "Changes to be committed:"
git diff config/deployments/default/kustomization.yaml charts/k8ssandra-operator/Chart.yaml

confirm "Do these changes look correct?"

# Commit the changes
git add config/deployments/default/kustomization.yaml charts/k8ssandra-operator/Chart.yaml
git commit -m "Prepare next patch release"

# Push the changes
git push ${GIT_REMOTE} ${RELEASE_BRANCH}

# Update the main branch
print_info "Step 11: Updating the main branch"
git checkout main

# Merge with "ours" strategy
print_info "Merging release branch to main (using 'ours' strategy)"
git merge ${RELEASE_BRANCH} -s ours

# Cherry-pick the changelog update
print_info "Cherry-picking changelog update"
CHANGELOG_COMMIT=$(git log ${RELEASE_BRANCH} --grep="Update changelog" --format="%H" -n 1)
if [ -z "$CHANGELOG_COMMIT" ]; then
    print_error "Could not find changelog commit"
    exit 1
fi
git cherry-pick -n ${CHANGELOG_COMMIT}

print_warning "DO NOT cherry-pick the 'Release vx.x.x' or 'Prepare next patch release' commits"
confirm "Have you verified only the changelog commit is staged?"

# Amend the merge commit
git add .
git commit --amend --no-edit

# Update version in main to next SNAPSHOT
print_info "Updating main branch to next version"
yq e -i ".version=env(NEXT_MAJOR_MINOR)+\".0-SNAPSHOT\"" charts/k8ssandra-operator/Chart.yaml

# Show the diff
print_info "Changes to be committed:"
git diff charts/k8ssandra-operator/Chart.yaml

confirm "Do these changes look correct?"

git add charts/k8ssandra-operator/Chart.yaml
git commit -m "Prepare next release"

# Create new blank changelog
print_info "Step 12: Creating new changelog file for next version"
NEW_CHANGELOG="CHANGELOG/CHANGELOG-${NEXT_MAJOR_MINOR}.md"

cat > ${NEW_CHANGELOG} << EOF
# Changelog

Changelog for the K8ssandra Operator, new PRs should update the \`unreleased\` section below with entries describing the changes like:

\`\`\`markdown
* [CHANGE]
* [FEATURE]
* [ENHANCEMENT]
* [BUGFIX]
* [DOCS]
* [TESTING]
\`\`\`

When cutting a new release, update the \`unreleased\` heading to the tag being generated and date, like \`## vX.Y.Z - YYYY-MM-DD\` and create a new placeholder section for \`unreleased\` entries.

## unreleased
EOF

git add charts/k8ssandra-operator/Chart.yaml ${NEW_CHANGELOG}
git commit -m "Prepare next release"

# Push all changes
print_info "Step 13: Pushing all changes to main"
git push ${GIT_REMOTE} main ${RELEASE_BRANCH}

print_info "=== Release process completed successfully! ==="
echo
print_info "Summary:"
echo "  - Release branch: ${RELEASE_BRANCH}"
echo "  - Release tag: v${RELEASED_MAJOR_MINOR}.0"
echo "  - Main branch updated to: ${NEXT_MAJOR_MINOR}.0-SNAPSHOT"
echo
print_info "Next steps:"
echo "  1. Verify the GitHub release is published"
echo "  2. Monitor for any issues with the release"
echo "  3. Update documentation if needed"

# Made with Bob
