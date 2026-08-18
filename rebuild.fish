#!/usr/bin/env fish
set -l repo (realpath (dirname (status filename)))

git -C "$repo" add -A # nix

set -l build
set -l ok 0
for attempt in 1 2
    set build (nix build "$repo" --out-link "$repo/result" 2>&1)
    set ok $status
    test $ok -eq 0; and break

    set -l got (printf '%s\n' $build | string replace -ra '\e\[[0-9;]*m' '' | string match -rg 'got:\s+(sha256-\S+)' | tail -1)
    if test -z "$got"; or test $attempt -eq 2
        break
    end
    echo "hash changed, patching... $got"
    sed -i 's|\(vendorHash = "\)[^"]*\("\)|\1'"$got"'\2|' "$repo/flake.nix"
end

if test $ok -ne 0
    printf '%s\n' $build >&2
    exit 1
end

systemctl --user stop justrayd 2>/dev/null # stop first, or systemd respawns it
pkill -x justrayd

exec "$repo/result/bin/justray" $argv
