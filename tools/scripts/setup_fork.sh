#!/usr/bin/env bash
set -e

# ==============================================================================
# Horizon SDV - Fork & Contribution Setup Script
# ==============================================================================
# This script automates the process of configuring your local environment
# to contribute to the Horizon SDV project via a personal fork.
#
# Usage: ./setup_fork.sh <YOUR_GITHUB_USERNAME>
# ==============================================================================

# 1. Validation
if [ -z "$1" ]; then
    echo "❌ Error: Please provide your GitHub username."
    echo "Usage: $0 <YOUR_GITHUB_USERNAME>"
    exit 1
fi

GITHUB_USERNAME=$1
UPSTREAM_URL="https://github.com/GoogleCloudPlatform/horizon-sdv.git"
FORK_URL="https://github.com/${GITHUB_USERNAME}/horizon-sdv.git"
CURRENT_BRANCH=$(git branch --show-current)

echo "🚀 Starting Fork Configuration for user: ${GITHUB_USERNAME}..."

# 2. Check current remotes
echo "--- Current Remotes ---"
git remote -v
echo "-----------------------"

# 3. Re-configure Origin to point to your Fork
echo "🔄 Setting 'origin' to your fork: ${FORK_URL}"
git remote set-url origin "${FORK_URL}"

# 4. Add Upstream to keep your fork synced with the original project
if ! git remote | grep -q "upstream"; then
    echo "➕ Adding 'upstream' remote: ${UPSTREAM_URL}"
    git remote add upstream "${UPSTREAM_URL}"
else
    echo "✅ 'upstream' remote already exists."
    git remote set-url upstream "${UPSTREAM_URL}"
fi

# 5. Final Verification
echo "--- Updated Remotes ---"
git remote -v
echo "-----------------------"

# 6. Push Instructions
echo ""
echo "✅ Configuration Complete!"
echo "----------------------------------------------------------------------"
echo "Next Steps:"
echo "1. Ensure you have clicked the 'Fork' button on the GitHub UI at:"
echo "   https://github.com/GoogleCloudPlatform/horizon-sdv"
echo ""
echo "2. Push your branch to your fork:"
echo "   git push -u origin ${CURRENT_BRANCH}"
echo ""
echo "3. Create a Pull Request:"
echo "   Navigate to your fork on GitHub and click 'Compare & pull request'."
echo "----------------------------------------------------------------------"
