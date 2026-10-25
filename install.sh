#!/bin/sh
# Install official release binaries; no Go, Node.js or Python is required.
set -eu
install_prefix="${BAB_PREFIX:-$HOME/.local}"
release_version=""
assets_dir=""
with_ttyd=false
site_origin=""
run_setup=true
while [ "$#" -gt 0 ]; do
 case "$1" in
  --prefix|--version|--from-dir|--origin)
   option=$1; shift
   [ "$#" -gt 0 ] && [ -n "$1" ] || { echo "$option requires a value" >&2; exit 2; }
   case "$option" in
    --prefix) install_prefix=$1;; --version) release_version=$1;;
    --from-dir) assets_dir=$1;; --origin) site_origin=$1;;
   esac;;
  --with)
   shift; [ "$#" -gt 0 ] && [ "$1" = ttyd ] || { echo 'Supported optional client: --with ttyd' >&2; exit 2; }
   with_ttyd=true;;
  --no-setup) run_setup=false;;
  --help)
   echo 'Usage: install.sh [--prefix PATH] [--version VERSION] [--with ttyd] [--origin URL] [--no-setup] [--from-dir ASSETS]'
   exit 0;;
  *) echo "Unknown option: $1" >&2; exit 2;;
 esac
 shift
done
[ -z "$site_origin" ] || [ "$with_ttyd" = true ] || { echo '--origin requires --with ttyd' >&2; exit 2; }
[ -z "$site_origin" ] || [ "$run_setup" = true ] || { echo '--origin cannot be used with --no-setup' >&2; exit 2; }
if [ -n "$assets_dir" ] && [ -z "$release_version" ]; then
 echo '--from-dir requires --version' >&2; exit 2
fi
case "$(uname -s)" in Darwin) release_os=darwin;; Linux) release_os=linux;; *) echo 'Supported systems: macOS and Linux' >&2; exit 2;; esac
case "$(uname -m)" in arm64|aarch64) release_arch=arm64;; x86_64|amd64) release_arch=amd64;; *) echo 'Supported architectures: arm64 and amd64' >&2; exit 2;; esac
workdir=$(mktemp -d "${TMPDIR:-/tmp}/bab-install.XXXXXX")
trap 'rm -rf "$workdir"' EXIT HUP INT TERM
download() { curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 "$1" -o "$2"; }
if [ -z "$release_version" ]; then
 # Prefer a published stable release; an Alpha-only project falls back to its
 # newest public release. A VERSION change on main never selects an absent asset.
 api='https://api.github.com/repos/sky-bro/browser-auth-bridge/releases'
 if ! download "$api/latest" "$workdir/release.json" 2>/dev/null; then
  download "$api?per_page=1" "$workdir/release.json" || { echo 'No published release is available; install from source or choose --version.' >&2; exit 1; }
 fi
 release_version=$(LC_ALL=C grep -Eo '"tag_name"[[:space:]]*:[[:space:]]*"v?[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?"' "$workdir/release.json" | head -n 1 | sed 's/^[^:]*:[[:space:]]*"//;s/"$//')
 [ -n "$release_version" ] || { echo 'No installable published release found; choose --version or install from source.' >&2; exit 1; }
fi
release_version=${release_version#v}
printf '%s\n' "$release_version" | LC_ALL=C grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$' || { echo 'Invalid release version' >&2; exit 2; }
asset="bab_${release_version}_${release_os}_${release_arch}.tar.gz"
if [ -n "$assets_dir" ]; then
 cp "$assets_dir/$asset" "$workdir/$asset"
 cp "$assets_dir/SHA256SUMS" "$workdir/SHA256SUMS"
else
 base="https://github.com/sky-bro/browser-auth-bridge/releases/download/v${release_version}"
 download "$base/$asset" "$workdir/$asset" || { echo "Release v$release_version is not available for $release_os/$release_arch." >&2; exit 1; }
 download "$base/SHA256SUMS" "$workdir/SHA256SUMS"
fi
expected=$(awk -v name="$asset" '$2 == name {print $1}' "$workdir/SHA256SUMS")
[ "${#expected}" -eq 64 ] || { echo 'Missing or ambiguous checksum' >&2; exit 1; }
case "$expected" in *[!0-9a-f]*) echo 'Invalid checksum' >&2; exit 1;; esac
if command -v sha256sum >/dev/null 2>&1; then
 actual=$(sha256sum "$workdir/$asset" | awk '{print $1}')
else
 actual=$(shasum -a 256 "$workdir/$asset" | awk '{print $1}')
fi
[ "$expected" = "$actual" ] || { echo 'Checksum mismatch; nothing installed.' >&2; exit 1; }
# Only expected regular programs and license notices may be extracted.
tar -tzf "$workdir/$asset" > "$workdir/members"
members=$(wc -l < "$workdir/members" | tr -d ' ')
[ "$members" -ge 2 ] && [ "$members" -le 5 ] || { echo 'Invalid release archive' >&2; exit 1; }
[ "$(grep -Ec '^(bab|bridge-ttyd|LICENSE|NOTICE|GO_LICENSE)$' "$workdir/members")" -eq "$members" ] || { echo 'Unexpected archive path' >&2; exit 1; }
tar -tvzf "$workdir/$asset" | awk 'substr($0,1,1) != "-" {bad=1} END {exit bad}' || { echo 'Release binaries must be regular files' >&2; exit 1; }
tar -xzf "$workdir/$asset" -C "$workdir" bab bridge-ttyd
mkdir -p "$install_prefix/bin"
install -m 755 "$workdir/bab" "$install_prefix/bin/.bab-install-$$"
mv -f "$install_prefix/bin/.bab-install-$$" "$install_prefix/bin/bab"
if [ "$with_ttyd" = true ]; then
 install -m 755 "$workdir/bridge-ttyd" "$install_prefix/bin/.ttyd-install-$$"
 mv -f "$install_prefix/bin/.ttyd-install-$$" "$install_prefix/bin/bridge-ttyd"
fi
for notice in LICENSE NOTICE GO_LICENSE; do
 if grep -qx "$notice" "$workdir/members"; then
  tar -xzf "$workdir/$asset" -C "$workdir" "$notice"
  mkdir -p "$install_prefix/share/doc/bab"
  install -m 644 "$workdir/$notice" "$install_prefix/share/doc/bab/$notice"
 fi
done
"$install_prefix/bin/bab" completion zsh --install --prefix "$install_prefix"
if [ "$run_setup" = true ]; then
 "$install_prefix/bin/bab" setup
 if [ "$with_ttyd" = true ] && [ -n "$site_origin" ]; then
  "$install_prefix/bin/bab" plugin list --json > "$workdir/installed.json"
  operation=install
  if grep -Eq '^[[:space:]]*"site": "ttyd",?$' "$workdir/installed.json"; then operation=update; fi
  PATH="$install_prefix/bin:$PATH" "$install_prefix/bin/bab" plugin "$operation" ttyd --origin "$site_origin"
 fi
fi
printf 'Installed BAB %s in %s/bin\n' "$release_version" "$install_prefix"
case ":$PATH:" in *":$install_prefix/bin:"*) ;; *) printf 'Add %s/bin to PATH, or invoke it by its full path.\n' "$install_prefix";; esac
