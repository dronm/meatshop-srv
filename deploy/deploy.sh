#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'USAGE'
Usage: deploy.sh [MODE] [--skip-migrations] [--dry-run]

Default mode deploys the MeatShop backend and admin frontend.
Modes:
  --backend-only       MeatShop backend
  --frontend-only      Admin frontend
  --max                MAX bot and mini-app frontend
  --max-backend-only   MAX bot
  --max-frontend-only  MAX mini-app frontend
  --all                Both backends and both frontends
  --migrations-only    Copy migrations and apply them to the primary
USAGE
}

die() {
	printf 'Deploy error: %s\n' "$*" >&2
	exit 1
}

mode=''
skip_migrations=false
dry_run=false
while (($#)); do
	case "$1" in
		--backend-only|--frontend-only|--max|--max-backend-only|--max-frontend-only|--all|--migrations-only)
			[[ -z $mode ]] || die 'Choose one deployment mode.'
			mode=$1
			;;
		--skip-migrations) skip_migrations=true ;;
		--dry-run) dry_run=true ;;
		--help|-h) usage; exit 0 ;;
		*) die "Unknown option $1. See --help." ;;
	esac
	shift
done

components=()
case "$mode" in
	'') components=(meatshop admin) ;;
	--backend-only) components=(meatshop) ;;
	--frontend-only) components=(admin) ;;
	--max) components=(max max-app) ;;
	--max-backend-only) components=(max) ;;
	--max-frontend-only) components=(max-app) ;;
	--all) components=(meatshop max admin max-app) ;;
	--migrations-only) ;;
esac

copy_migrations=true
case "$mode" in
	--frontend-only|--max-frontend-only) copy_migrations=false ;;
	--migrations-only)
		[[ $skip_migrations == false ]] || die '--migrations-only requires applying migrations.'
		;;
esac

run_migrations=false
if [[ $copy_migrations == true && $skip_migrations == false ]]; then
	run_migrations=true
fi

deploy_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
root_dir=$(cd -- "$deploy_dir/.." && pwd -P)
config=${DEPLOY_CONFIG:-$deploy_dir/deploy.conf}
[[ -r $config ]] || die "Copy deploy/deploy.conf.example to $config and fill in your settings."

declare -a SERVERS=()
declare -a ADMIN_BUILD_CMD=(npm run build)
declare -a MAX_APP_BUILD_CMD=(npm run build -- --base=/max-app/)
# The local config is a trusted Bash file.
source "$config"

valid_remote_path() {
	[[ $1 == /* && $1 != */ && $1 != *//* && $1 != *[[:cntrl:]]* &&
		$1 != */./* && $1 != */../* && $1 != */. && $1 != */.. ]]
}

paths_overlap() {
	[[ $1 == "$2" || $1 == "$2/"* || $2 == "$1/"* ]]
}

(("${#SERVERS[@]}" > 0)) || die 'Set SERVERS to at least one SSH destination.'
declare -A seen_servers=()
for server in "${SERVERS[@]}"; do
	[[ $server =~ ^([[:alnum:]_][[:alnum:]_.-]*@)?[[:alnum:]_][[:alnum:]_.-]*$ ]] ||
		die "Invalid SSH destination: $server. Use an SSH alias or user@hostname."
	[[ ! ${seen_servers[$server]+set} ]] || die "Duplicate SSH destination: $server."
	seen_servers[$server]=1
done

migration_host=${MIGRATION_HOST:-${SERVERS[0]}}
[[ ${seen_servers[$migration_host]+set} ]] || die 'MIGRATION_HOST must be in SERVERS.'
valid_remote_path "${REMOTE_RELEASES_DIR:-}" || die 'Set REMOTE_RELEASES_DIR to an absolute directory path.'
systemctl_mode=${SYSTEMCTL_MODE:-sudo}
[[ $systemctl_mode == sudo || $systemctl_mode == direct ]] || die 'SYSTEMCTL_MODE must be sudo or direct.'
goarch=${DEPLOY_GOARCH:-amd64}

declare -A active_paths=(
	[meatshop]="${REMOTE_MEATSHOP_PATH:-}"
	[max]="${REMOTE_MAX_PATH:-}"
	[admin]="${REMOTE_ADMIN_PATH:-}"
	[max-app]="${REMOTE_MAX_APP_PATH:-}"
)
declare -A units=(
	[meatshop]="${MEATSHOP_UNIT:-meatshop.service}"
	[max]="${MAX_UNIT:-max.service}"
	[admin]=''
	[max-app]=''
)
declare -A health_urls=(
	[meatshop]="${MEATSHOP_HEALTH_URL:-}"
	[max]="${MAX_HEALTH_URL:-}"
	[admin]="${ADMIN_HEALTH_URL:-}"
	[max-app]="${MAX_APP_HEALTH_URL:-}"
)
declare -A frontend_dirs=(
	[admin]="${ADMIN_DIR:-}"
	[max-app]="${MAX_APP_DIR:-}"
)
declare -A frontend_outputs=(
	[admin]="${ADMIN_DIST_DIR:-dist}"
	[max-app]="${MAX_APP_DIST_DIR:-dist}"
)

configured_paths=("$REMOTE_RELEASES_DIR")
for component in meatshop max admin max-app; do
	active_path=${active_paths[$component]}
	[[ -n $active_path ]] || continue
	valid_remote_path "$active_path" || die "Set an absolute publish path for $component."
	for path in "${configured_paths[@]}"; do
		if paths_overlap "$active_path" "$path"; then
			die "Remote paths overlap: $active_path and $path."
		fi
	done
	configured_paths+=("$active_path")
done

for component in "${components[@]}"; do
	[[ -n ${active_paths[$component]} ]] || die "Set the remote publish path for $component."
	case "$component" in
		meatshop|max)
			[[ ${units[$component]} =~ ^[a-zA-Z0-9_][a-zA-Z0-9_.@:-]*\.service$ ]] ||
				die "Set a valid systemd service name for $component."
			;;
		admin|max-app)
			frontend_dir=${frontend_dirs[$component]}
			[[ -n $frontend_dir ]] || die "Set the local project directory for $component."
			if [[ $frontend_dir != /* ]]; then
				frontend_dir=$root_dir/$frontend_dir
			fi
			output_dir=${frontend_outputs[$component]}
			if [[ $output_dir != /* ]]; then
				output_dir=$frontend_dir/$output_dir
			fi
			frontend_dirs[$component]=$frontend_dir
			frontend_outputs[$component]=$output_dir
			;;
	esac
done

if [[ $mode == --all && ${units[meatshop]} == "${units[max]}" ]]; then
	die 'Set different systemd units for MeatShop and MAX.'
fi

if [[ $copy_migrations == true ]]; then
	[[ -d $root_dir/migrations ]] || die "Migration directory missing: $root_dir/migrations."
	if ! compgen -G "$root_dir/migrations/*.up.sql" >/dev/null; then
		die "No versioned up migrations found in $root_dir/migrations."
	fi
fi

migrate_bin=${MIGRATE_BIN:-migrate}
if [[ $run_migrations == true ]]; then
	valid_remote_path "${REMOTE_DB_ENV_FILE:-}" || die 'Set REMOTE_DB_ENV_FILE to the absolute path of the primary DB config.'
	[[ $migrate_bin != -* ]] || die 'Set MIGRATE_BIN to a command or absolute path.'
fi

printf 'Servers: %s\n' "${SERVERS[*]}"
for component in "${components[@]}"; do
	printf 'Deploy %s to %s\n' "$component" "${active_paths[$component]}"
done
if [[ $copy_migrations == true ]]; then
	printf 'Upload the complete migrations/ tree to each server.\n'
fi
if [[ $run_migrations == true ]]; then
	printf 'Apply pending migrations once from %s.\n' "$migration_host"
fi
if [[ $dry_run == true ]]; then
	exit 0
fi

# SSH passes a command string to the remote login shell. Quote each argument
# for that shell before the remote Bash script receives it.
remote_bash() {
	local server=$1
	local command='bash -s --'
	local argument
	shift
	for argument in "$@"; do
		command+=" '${argument//\'/\'\\\'\'}'"
	done
	ssh -o BatchMode=yes -- "$server" "$command"
}

remote_operation() {
	local server=$1
	shift
	remote_bash "$server" "$@" <<'REMOTE'
set -euo pipefail
umask 022
action=$1
shift
case "$action" in
	lock)
		root=$1
		token=$2
		mkdir -p -- "$root"
		if ! mkdir -- "$root/.deploy-lock"; then
			printf 'Could not acquire deployment lock at %s/.deploy-lock.\n' "$root" >&2
			exit 1
		fi
		printf '%s\n' "$token" > "$root/.deploy-lock/owner"
		;;
	unlock)
		root=$1
		token=$2
		lock_dir=$root/.deploy-lock
		if [[ -f $lock_dir/owner && $(<"$lock_dir/owner") == "$token" ]]; then
			rm -- "$lock_dir/owner"
			rmdir -- "$lock_dir"
		fi
		;;
	init)
		command -v rsync >/dev/null || { echo 'rsync is required on each server.' >&2; exit 1; }
		mkdir -p -- "${1%/*}"
		mkdir -- "$1"
		;;
	check-migration|migrate)
		migration_dir=$1
		migration_config=$2
		migration_cli=$3
		[[ -r $migration_config ]] || { printf 'Cannot read %s.\n' "$migration_config" >&2; exit 1; }
		command -v "$migration_cli" >/dev/null ||
			{ printf 'Cannot find migration CLI %s.\n' "$migration_cli" >&2; exit 1; }
		unset DB_CONN
		source "$migration_config"
		[[ -n ${DB_CONN:-} ]] || { echo 'The remote migration config must set DB_CONN.' >&2; exit 1; }
		if [[ $action == migrate ]]; then
			"$migration_cli" -path "$migration_dir" -database "$DB_CONN" up
		fi
		;;
esac
REMOTE
}

build_backend() {
	local component=$1
	local destination=$2
	mkdir -- "$destination"
	case "$component" in
		meatshop)
			GOOS=linux GOARCH="$goarch" make -C "$root_dir" prod "BUILD_OUTPUT=$destination/meatshop"
			;;
		max)
			GOOS=linux GOARCH="$goarch" make -C "$root_dir" prod-max "MAX_BUILD_OUTPUT=$destination/max"
			;;
	esac
	[[ -x $destination/$component ]] || die "Build did not produce an executable for $component."
}

build_frontend() {
	local component=$1
	local build_command=$2
	local -n command_args=$build_command
	local source_dir output_dir
	(("${#command_args[@]}" > 0)) || die "Set the build command for $component."
	source_dir=$(cd -- "${frontend_dirs[$component]}" && pwd -P) ||
		die "Local project directory is unavailable for $component."
	(
		cd -- "$source_dir"
		"${command_args[@]}"
	)
	output_dir=$(cd -- "${frontend_outputs[$component]}" && pwd -P) ||
		die "Build output directory is unavailable for $component."
	if [[ $output_dir == "$source_dir" || $source_dir == "${output_dir%/}/"* ||
		$output_dir == "$root_dir" || $root_dir == "${output_dir%/}/"* ]]; then
		die "Set a dedicated build output directory for $component."
	fi
	[[ -s $output_dir/index.html ]] || die "No built index.html found for $component."
	frontend_outputs[$component]=$output_dir
}

upload_tree() {
	local server=$1
	local source=$2
	local destination=$3
	rsync -azs --chmod=Du=rwx,Dgo=rx,Fu=rwX,Fgo=rX -e 'ssh -o BatchMode=yes' -- "$source/" "$server:$destination/"
}

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/meatshop-release.XXXXXXXX")
release_id=$(date -u +%Y%m%dT%H%M%SZ)-${work_dir##*.}
release_dir=$REMOTE_RELEASES_DIR/$release_id
lock_requested=false
cleanup() {
	local status=$?
	trap - EXIT
	trap '' INT TERM
	if [[ $lock_requested == true ]]; then
		if ((status == 130 || status == 143 || status == 255)); then
			printf 'Deployment interrupted. Check the deployment lock on %s before retrying.\n' "$migration_host" >&2
		elif ! remote_operation "$migration_host" unlock "$REMOTE_RELEASES_DIR" "$release_id"; then
			printf 'Could not release the deployment lock on %s.\n' "$migration_host" >&2
			if ((status == 0)); then status=1; fi
		fi
	fi
	rm -rf -- "$work_dir"
	if ((status != 0)); then
		printf 'Deployment failed. Release ID: %s\n' "$release_id" >&2
	fi
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

lock_requested=true
remote_operation "$migration_host" lock "$REMOTE_RELEASES_DIR" "$release_id"
printf 'Staging release %s\n' "$release_id"
if [[ $run_migrations == true ]]; then
	remote_operation "$migration_host" check-migration "$release_dir/migrations" "$REMOTE_DB_ENV_FILE" "$migrate_bin"
fi
for server in "${SERVERS[@]}"; do
	remote_operation "$server" init "$release_dir"
	for component in "${components[@]}"; do
		remote_bash "$server" preflight "$release_dir/$component" "${active_paths[$component]}" \
			"${units[$component]}" "${health_urls[$component]}" "$systemctl_mode" < "$deploy_dir/remote.sh"
	done
done

declare -A artifacts=()
for component in "${components[@]}"; do
	case "$component" in
		meatshop|max)
			artifacts[$component]=$work_dir/$component
			build_backend "$component" "${artifacts[$component]}"
			;;
		admin)
			build_frontend "$component" ADMIN_BUILD_CMD
			artifacts[$component]=${frontend_outputs[$component]}
			;;
		max-app)
			build_frontend "$component" MAX_APP_BUILD_CMD
			artifacts[$component]=${frontend_outputs[$component]}
			;;
	esac
done

if [[ $mode == --all ]] && paths_overlap "${artifacts[admin]}" "${artifacts[max-app]}"; then
	die 'Set separate build output directories for the admin and MAX frontends.'
fi

for server in "${SERVERS[@]}"; do
	printf 'Uploading release to %s\n' "$server"
	for component in "${components[@]}"; do
		upload_tree "$server" "${artifacts[$component]}" "$release_dir/$component"
	done
	if [[ $copy_migrations == true ]]; then
		upload_tree "$server" "$root_dir/migrations" "$release_dir/migrations"
	fi
done

if [[ $run_migrations == true ]]; then
	printf 'Applying migrations from %s\n' "$migration_host"
	remote_operation "$migration_host" migrate "$release_dir/migrations" "$REMOTE_DB_ENV_FILE" "$migrate_bin"
fi

for component in "${components[@]}"; do
	for server in "${SERVERS[@]}"; do
		printf 'Activating %s on %s\n' "$component" "$server"
		remote_bash "$server" activate "$release_dir/$component" "${active_paths[$component]}" \
			"${units[$component]}" "${health_urls[$component]}" "$systemctl_mode" < "$deploy_dir/remote.sh"
	done
done
printf 'Deployed release %s\n' "$release_id"
