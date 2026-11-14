#!/usr/bin/env bash
set -euo pipefail

log() {
	printf '[%s] %s\n' "${HOSTNAME:-remote}" "$*" >&2
}

fail() {
	log "$*"
	exit 1
}

if (( $# != 6 )); then
	fail 'Expected action, release directory, active path, unit, health URL, and sudo mode.'
fi

action=$1
release_dir=$2
active_path=$3
unit=$4
health_url=$5
sudo_mode=$6

for path in "$release_dir" "$active_path"; do
	[[ "$path" == /* && "$path" != */ && "$path" != *//* &&
		"$path/" != */../* && "$path/" != */./* ]] || fail "Invalid remote path: $path"
done

[[ "$sudo_mode" == sudo || "$sudo_mode" == direct ]] || fail "Invalid sudo mode: $sudo_mode"
[[ -z "$unit" || "$unit" =~ ^[a-zA-Z0-9_][a-zA-Z0-9_.@:-]*\.service$ ]] || fail "Invalid service unit: $unit"
[[ -z "$health_url" || "$health_url" == http://* || "$health_url" == https://* ]] || fail 'Health URL must use HTTP or HTTPS.'

release_parent=${release_dir%/*}
active_parent=${active_path%/*}
release_parent=${release_parent:-/}
active_parent=${active_parent:-/}

check_separation() {
	local release=$1 active=$2
	[[ "$release" != "$active" && "$release" != "$active/"* && "$active" != "$release/"* ]] ||
		fail "Release and active paths overlap: $release and $active"
}

check_layout() {
	local parent physical_release_parent physical_active_parent

	[[ ! -e "$active_path" || -L "$active_path" ]] ||
		fail "Active path exists and is not a symlink: $active_path"
	for parent in "$release_parent" "$active_parent"; do
		[[ -d "$parent" && -w "$parent" && -x "$parent" ]] || fail "Directory is not writable: $parent"
	done
	physical_release_parent=$(cd -P -- "$release_parent" && pwd -P)
	physical_active_parent=$(cd -P -- "$active_parent" && pwd -P)
	check_separation "${physical_release_parent%/}/${release_dir##*/}" "${physical_active_parent%/}/${active_path##*/}"
}

systemctl_bin=''
if [[ -n "$unit" ]]; then
	systemctl_bin=$(command -v systemctl) || fail 'systemctl is required for backend deployment.'
fi

service_action() {
	if [[ "$sudo_mode" == sudo ]]; then
		sudo -n -- "$systemctl_bin" "$1" "$unit"
	else
		"$systemctl_bin" --no-ask-password "$1" "$unit"
	fi
}

preflight() {
	command -v rsync >/dev/null || fail 'rsync is required on the deployment server.'
	mv --version >/dev/null 2>&1 || fail 'GNU mv is required on the deployment server.'
	if [[ -n "$health_url" ]]; then
		command -v curl >/dev/null || fail 'curl is required when a health URL is configured.'
	fi
	if [[ -n "$unit" ]]; then
		local load_state
		load_state=$("$systemctl_bin" show --property=LoadState --value "$unit") || fail "Cannot inspect $unit"
		[[ "$load_state" == loaded ]] || fail "Service $unit has load state '$load_state'."
		if [[ "$sudo_mode" == sudo ]]; then
			sudo -n -l -- "$systemctl_bin" restart "$unit" >/dev/null || fail "Passwordless restart is required for $unit."
			sudo -n -l -- "$systemctl_bin" stop "$unit" >/dev/null || fail "Passwordless stop is required for $unit."
		fi
	fi
	[[ ! -e "$release_dir" && ! -L "$release_dir" ]] || fail "Release path already exists: $release_dir"
	mkdir -p -- "$release_parent" "$active_parent"
	check_layout
}

wait_for_health() {
	local attempt
	for (( attempt = 1; attempt <= 8; attempt++ )); do
		if { [[ -z "$unit" ]] || "$systemctl_bin" is-active --quiet "$unit"; } &&
			{ [[ -z "$health_url" ]] || curl -q --fail --silent --output /dev/null --max-time 2 -- "$health_url"; }; then
			return 0
		fi
		if (( attempt < 8 )); then
			sleep 1
		fi
	done
	return 1
}

link_dir=''
previous_target=''
rollback_needed=false

switch_link() {
	ln -s -- "$1" "$link_dir/active" && mv -Tf -- "$link_dir/active" "$active_path"
}

cleanup_activation() {
	local status=$?
	trap '' HUP INT TERM
	if [[ "$rollback_needed" == true && "$(readlink -- "$active_path" 2>/dev/null)" == "$release_dir" ]]; then
		log "Activation failed for $active_path. Restoring its previous state."
		if [[ -n "$previous_target" ]]; then
			if ! switch_link "$previous_target"; then
				log "Could not restore $active_path. Previous target: $previous_target"
			elif [[ -n "$unit" ]] && ! service_action restart; then
				log "Restored $active_path, but could not restart $unit."
			fi
		else
			if [[ -n "$unit" ]] && ! service_action stop; then
				log "Could not stop $unit after the failed first activation."
			fi
			rm -f -- "$active_path" || log "Could not remove $active_path."
		fi
	fi
	rm -f -- "$link_dir/active" || true
	rmdir -- "$link_dir" || true
	exit "$status"
}

activate() {
	check_layout
	[[ -d "$release_dir" && ! -L "$release_dir" && -r "$release_dir" && -x "$release_dir" ]] ||
		fail "Staged release directory is unavailable: $release_dir"
	if [[ -L "$active_path" ]]; then
		previous_target=$(readlink -- "$active_path") || fail "Cannot read $active_path"
	fi

	link_dir=$(mktemp -d -- "${active_parent%/}/.deploy-link.XXXXXXXX")
	trap cleanup_activation EXIT
	trap 'exit 129' HUP
	trap 'exit 130' INT
	trap 'exit 143' TERM

	rollback_needed=true
	switch_link "$release_dir" || fail "Could not activate $release_dir at $active_path"
	if [[ -n "$unit" ]]; then
		service_action restart || fail "Could not restart $unit"
	fi
	wait_for_health || fail "Health check failed for ${unit:-$active_path}."
	rollback_needed=false
	log "Activated $active_path -> $release_dir"
}

check_separation "$release_dir" "$active_path"
case "$action" in
	preflight) preflight ;;
	activate) activate ;;
	*) fail "Unknown remote deployment action: $action" ;;
esac
