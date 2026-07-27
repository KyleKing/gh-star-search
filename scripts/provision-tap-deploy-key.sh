#!/usr/bin/env bash
#
# provision-tap-deploy-key.sh - one-time setup for goreleaser's Homebrew tap push.
#
# GitHub has no API for creating personal access tokens, so the tap push uses a
# deploy key instead: goreleaser pushes the cask over SSH, authenticated by a
# key that exists only on the tap repo. This script generates the keypair, adds
# the public half as a write-enabled deploy key on the tap repo, archives the
# private half in 1Password, and stores it as the TAP_DEPLOY_KEY Actions secret
# on the current repo (the name .goreleaser.yml and the release workflow expect).
# Re-running rotates the key: the old deploy key with the same title is replaced
# and the secret is overwritten.
#
# Usage: provision-tap-deploy-key.sh [tap-repo]
#   tap-repo  defaults to <owner>/homebrew-tap for the current repo's owner
#             and must already exist
# Environment:
#   OP_VAULT  1Password vault for the archived key (default: Private)

set -euo pipefail

readonly SECRET_NAME="TAP_DEPLOY_KEY"
OP_VAULT="${OP_VAULT:-Private}"

for tool in gh op; do
    command -v "$tool" >/dev/null || { echo "Error: $tool not found on PATH" >&2; exit 1; }
done

target_repo="$(gh repo view --json nameWithOwner -q .nameWithOwner)"
owner="${target_repo%%/*}"
tap_repo="${1:-$owner/homebrew-tap}"
title="goreleaser tap push from $target_repo"

if ! gh repo view "$tap_repo" >/dev/null 2>&1; then
    echo "Error: $tap_repo does not exist; create it first: gh repo create $tap_repo --public" >&2
    exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
ssh-keygen -q -t ed25519 -N "" -C "$title" -f "$tmp/id_ed25519"

gh repo deploy-key list --repo "$tap_repo" --json id,title \
    -q ".[] | select(.title == \"$title\") | .id" |
    while read -r key_id; do
        echo "Removing existing deploy key $key_id"
        gh repo deploy-key delete --repo "$tap_repo" "$key_id"
    done

gh repo deploy-key add --repo "$tap_repo" --allow-write --title "$title" "$tmp/id_ed25519.pub"
echo "Added write deploy key to $tap_repo"

op item create --category ssh --vault "$OP_VAULT" --title "$title" \
    "private key[file]=$tmp/id_ed25519" >/dev/null
echo "Archived private key in 1Password vault '$OP_VAULT' as '$title'"

gh secret set "$SECRET_NAME" --repo "$target_repo" <"$tmp/id_ed25519"
echo "Set Actions secret $SECRET_NAME on $target_repo"
