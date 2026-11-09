# MeatShop deployment

Run deployment from the backend repository on your development computer. The script builds locally, then connects to the servers in `SERVERS` through SSH. All servers use the same remote paths and service names. MeatShop and MAX share one PostgreSQL connection to the primary.

## One-time setup

1. Copy `deploy/deploy.conf.example` to `deploy/deploy.conf` and edit it. The local config is excluded from Git. Set `DEPLOY_CONFIG` if you keep it elsewhere.
2. Fill `SERVERS` with the SSH aliases or `user@host` addresses you already use. All entries receive the components selected by the deploy command. The migration and lock coordinator defaults to the first entry. Set `MIGRATION_HOST` to choose another entry from the same list.
3. Set the remote release root and the active paths for both backends and both frontends. Use absolute paths. The SSH user needs permission to create and write release directories and update the active symlinks. The remote hosts need Bash, rsync, GNU coreutils, and systemd for backend services. Set `DEPLOY_GOARCH` if your servers use a Go architecture other than the default `amd64`.
4. Install `golang-migrate` with PostgreSQL support on `MIGRATION_HOST`. Set `MIGRATE_BIN` if its executable is outside the default SSH command path. Create a shell config file there at `REMOTE_DB_ENV_FILE`, readable by the SSH user and protected from other users. Set `DB_CONN` in that file to the PostgreSQL URL for the primary. The deploy command reads that file on the migration host.
5. Set each frontend's local project directory, build command array, and dedicated build output directory. Project directories can be absolute or relative to the backend repository. Output directories can be absolute or relative to the corresponding frontend project. The MAX app must be built with `/max-app/` as its public base path. The build command runs from the project directory and its output needs to contain `index.html`.

## Service and web server paths

Point each existing systemd unit at the executable under its configured active symlink. For example, if `REMOTE_MEATSHOP_PATH=/srv/meatshop/backend` and `REMOTE_MAX_PATH=/srv/meatshop/max`, use `/srv/meatshop/backend/meatshop` and `/srv/meatshop/max/max` as the corresponding `ExecStart` executable paths. Keep runtime configuration and the service working directory in persistent locations outside the release tree.

The default `SYSTEMCTL_MODE=sudo` uses `sudo -n` for systemd changes. Grant the SSH user passwordless access to `systemctl restart` and `systemctl stop` for the selected units. Use `SYSTEMCTL_MODE=direct` when the SSH user already has permission to perform those operations. Set optional health URLs to check HTTP from each target server. Those servers need `curl` when a health URL is set.

Point the web server at `REMOTE_ADMIN_PATH` for the admin frontend and `REMOTE_MAX_APP_PATH` for the mini-app served under `/max-app/`. The deploy script accepts active paths that are absent or already symlinks. If you already deploy into real directories, choose separate active paths and seed those symlinks with the existing deployments before switching the systemd and web server configs to use them.

## Commands

| Command | Components |
| --- | --- |
| `make deploy` or `make deploy-full` | MeatShop backend and admin frontend |
| `make deploy-backend` | MeatShop backend |
| `make deploy-frontend` | Admin frontend |
| `make deploy-max` | MAX bot and mini-app frontend |
| `make deploy-max-backend` | MAX bot |
| `make deploy-max-frontend` | MAX mini-app frontend |
| `make deploy-all` | Both backends and both frontends |
| `make deploy-migrations` | Migrations on their own |
| `make deploy-no-migrations` | MeatShop backend and admin frontend with migration execution skipped |
| `make deploy-plan` | Print the full MeatShop deployment plan |

You can preview other commands with `--dry-run` and skip migration execution with `--skip-migrations` when deploying a backend. For example, run `bash deploy/deploy.sh --all --dry-run` or `bash deploy/deploy.sh --max --skip-migrations`.

## Rollout and recovery

The script stages every selected component on all servers before activation. A backend or migrations-only deploy also stages the complete `migrations/` directory on every server, including `migrations/sql/`. The migration runner calls `migrate up` once before activation unless you skip migration execution. SQL helpers in `migrations/sql/` are copied for explicit use. Frontend-only commands publish their frontend assets.

Activation switches each component's symlink to the new release. If its restart or health check fails, the script attempts to restore the previous symlink on that server and stops the rollout. Earlier activations may still be live. Keep those release directories and their predecessors available until the rollout has been verified. Database recovery is separate from application rollback. Because both backends share the database, migrations must support the versions running during a rolling deployment. Coordinate a maintenance window when a migration requires the two services to change together.

All deploy commands that change a server use a shared lock at `.deploy-lock` inside `REMOTE_RELEASES_DIR` on `MIGRATION_HOST`. If a cancelled or interrupted deployment leaves the lock behind, verify that no deployment is running, then remove its `owner` file and the empty lock directory on that host before retrying.
