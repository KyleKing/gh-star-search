#!/usr/bin/env bash
#
# provision-tap-deploy-key.sh - one-time setup for goreleaser's Homebrew tap push.
#
# GitHub has no API for creating personal access tokens, so the tap push uses a
# deploy key instead: goreleaser pushes the cask over SSH, authenticated by a
# key that exists only on the tap repo. This script has 1Password generate the
# keypair (an item created with `--ssh-generate-key` is one it can read back;
# one built from `item create --template` with a raw SSHKEY-typed field is not,
# confirmed against 1Password CLI 2.39.0 across several existing items), adds
# the public half as a write-enabled deploy key on the tap repo, and stores the
# private half as the TAP_DEPLOY_KEY Actions secret on the current repo (the
# name .goreleaser.yml and the release workflow expect). Re-running rotates the
# key: the old deploy key and 1Password item with the same title are replaced,
# and the secret is overwritten.
#
# Usage: provision-tap-deploy-key.sh [tap-repo]
#   tap-repo  defaults to <owner>/homebrew-tap for the current repo's owner
#             and must already exist
# Environment:
#   OP_VAULT  1Password vault for the archived key (default: Private)
#   GH_TOKEN  optional override for the token gh uses; recommended if your
#             default gh auth token is shared across repos, since the token
#             running this script needs elevated, repo-scoped permissions:
#               tap repo:     Administration: Read and write (deploy keys)
#               current repo: Secrets: Read and write (Actions secret)
#             A fine-grained PAT limited to just these two repos with just
#             these two permissions avoids granting Administration on every
#             repo your everyday token can reach.

set -euo pipefail

readonly SECRET_NAME="TAP_DEPLOY_KEY"
OP_VAULT="${OP_VAULT:-Private}"

for tool in gh op python3; do
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

gh repo deploy-key list --repo "$tap_repo" --json id,title \
    -q ".[] | select(.title == \"$title\") | .id" |
    while read -r key_id; do
        echo "Removing existing deploy key $key_id"
        gh repo deploy-key delete --repo "$tap_repo" "$key_id"
    done

op item list --vault "$OP_VAULT" --format json |
    python3 -c '
import json, sys
title = sys.argv[1]
for item in json.load(sys.stdin):
    if item["title"] == title:
        print(item["id"])
' "$title" |
    while read -r item_id; do
        echo "Removing existing 1Password item $item_id"
        op item delete --vault "$OP_VAULT" "$item_id"
    done

item_id="$(op item create --category="SSH Key" --title="$title" \
    --vault "$OP_VAULT" --ssh-generate-key=ed25519 --format json |
    python3 -c 'import json, sys; print(json.load(sys.stdin)["id"])')"
echo "Generated SSH key in 1Password vault '$OP_VAULT' as '$title'"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

op read "op://$OP_VAULT/$item_id/public key" >"$tmp/id_ed25519.pub"
gh repo deploy-key add --repo "$tap_repo" --allow-write --title "$title" "$tmp/id_ed25519.pub"
echo "Added write deploy key to $tap_repo"

op read "op://$OP_VAULT/$item_id/private key?ssh-format=openssh" |
    gh secret set "$SECRET_NAME" --repo "$target_repo"
echo "Set Actions secret $SECRET_NAME on $target_repo"
