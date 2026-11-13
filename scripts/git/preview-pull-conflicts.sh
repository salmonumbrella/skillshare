#!/usr/bin/env bash
# Run inside the devcontainer image. Uses temporary Git repos and isolated XDG paths.
set -euo pipefail

cd "$(dirname "$0")/../.."
task_root=$(mktemp -d "${TMPDIR:-/tmp}/skillshare-pull-ui.XXXXXX")
export XDG_CONFIG_HOME="$task_root/config"
export XDG_DATA_HOME="$task_root/data"
export XDG_STATE_HOME="$task_root/state"
export XDG_CACHE_HOME="$task_root/cache"
mkdir -p "$task_root/source/shared" "$XDG_CONFIG_HOME/skillshare" "$XDG_CACHE_HOME/skillshare/ui"

git init -q -b main "$task_root/source"
git -C "$task_root/source" config user.name 'UI Test'
git -C "$task_root/source" config user.email 'ui-test@example.invalid'
cat > "$task_root/source/shared/SKILL.md" <<'EOF'
---
name: shared
description: A skill edited on two computers
---
# Shared skill

Original instructions.
EOF
printf 'Original notes.\n' > "$task_root/source/shared/notes.md"
git -C "$task_root/source" add -A
git -C "$task_root/source" commit -qm 'Initial skills'
git init -q --bare -b main "$task_root/remote.git"
git -C "$task_root/source" remote add origin "$task_root/remote.git"
git -C "$task_root/source" push -qu origin main
git clone -q "$task_root/remote.git" "$task_root/other"
git -C "$task_root/other" config user.name 'Other Computer'
git -C "$task_root/other" config user.email 'other-test@example.invalid'
sed -i 's/Original instructions./Instructions updated on the other computer./' "$task_root/other/shared/SKILL.md"
printf 'Notes from the other computer.\n' > "$task_root/other/shared/notes.md"
git -C "$task_root/other" commit -qam 'Other computer updates'
git -C "$task_root/other" push -q
sed -i 's/Original instructions./Instructions updated on this computer./' "$task_root/source/shared/SKILL.md"
printf 'Notes from this computer.\n' > "$task_root/source/shared/notes.md"
git -C "$task_root/source" commit -qam 'Local updates'
printf 'source: %s/source\nmode: merge\ntargets: {}\n' "$task_root" > "$XDG_CONFIG_HOME/skillshare/config.yaml"
ln -s "$PWD/ui/dist" "$XDG_CACHE_HOME/skillshare/ui/dev"
go build -o "$task_root/skillshare" ./cmd/skillshare
printf 'Isolated fixture: %s\n' "$task_root"
exec "$task_root/skillshare" ui -g --host 0.0.0.0 --port "${SKILLSHARE_PREVIEW_PORT:-49421}" --no-open
