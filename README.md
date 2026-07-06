# git webhook receiver

A small service that listens for incoming webhook HTTP POST requests from a Git
provider (Gitea, GitHub, GitLab) for one or more projects and runs a given
script in response to matching webhook events.

Its intended use is to run CI/CD scripts on a server, but it can be used to
execute arbitrary actions on Git events.

In a nutshell:

- [Deploy](#installation) it to your server.
- (Optional but recommended) Install an SSL certificate or wrap it with a
  [reverse proxy](./docs/ssl-or-nginx-setup.md#nginx-configuration), such as
  Nginx or caddy, to enable encryption.
- Create a [config file](#config-file) and add your projects along with their
  corresponding build scripts/actions, either as standalone scripts or
  [cross-platform inline scripts](#inline-scripts-and-standalone-scripts)
- Optional: add the app to your server startup scripts (systemd scripts
  setup is described [in this document](./docs/systemd-init-script.md))
- set webhooks for those repo in their git services (Github, Gitea, Gitlab,
  etc.) to post to {YOUR_HOST}/projects/{PROJECT_NAME}. Information on how to
  setup github webhooks for your project can be found
  in [this document](./docs/github-webhooks-setup.md).
- Start the service. It will listen for the webhook posts and execute the
  actions described in the config (e.g., building your projects or performing
  other tasks) when the webhooks are triggered by Git.

WIP, but operational. Some planned MVP functionality is still missing and there
will be breaking changes before version 1.0 release.

## Config file

The app requires a config file to start. By default, the config file is read
from `./config.yml`. You can override this behavior by setting `CONFIG_PATH` env
variable or by using `--config` flag when launching the app (the flag takes
precedence).

A typical config file may look like this:

```yaml
# provide it through env AUTH_PASSWORD, if you don't want it readable in config
auth_password: "password for inspection api and web ui basic auth"
projects:
  my_awesome_project:
    repo: "username/reponame"
    # generate it with `openssl rand -base64 42`
    # to supply through env: PROJECTS__my_awesome_project__SECRET=foo
    secret: "YourSecretGoesHere"
    actions:
      - on: push
        branch: main
        user: www-data # requires elevated permissions, consider if you need it
        cwd: "/var/www/default"
        script: |
          git fetch && git reset --hard origin/main
          npm ci --ignore-scripts
          npm run build
      - branch: master # fancy clone to temp dir
        with_temp_dir: true
        cwd: "/var/www/fancy-clone"
        environment:
          - GIT_TOKEN=github_pat_blahblah
        script: |
          git clone --depth 1 https://${GIT_TOKEN}@github.com/${GIT_REPO} "$TMPDIR"
          cd "$TMPDIR"
          npm ci --ignore-scripts
          npm run build
          cp -r dist/* "$CWD"
  my_other_project:
    repo: "username/reponame2"
    secret: "YourSecretGoesHere"
    # to supply through env: PROJECTS__my_other_project__AUTH=foo
    auth: "Your Authorization header key if you want"
    provider: gitea
    actions:
      - cwd: "/var/www/backend" # Anything besides `script` or `run` is optional
        run: ["sh", "./build.sh"]
```

Please refer to the [config file example](./config.example.yml) in this repo, to
see a list of all available configuration options.

For security reason, it's recommended to always provide a `secret` or at least
`authorization` param (in case of Gitea or Gitlab providers) for every action.
The `secret` also protects against MiM attacks, ensuring that the payload
hasn't been tampered with.

Gitlab currently
[doesn't sign](https://gitlab.com/gitlab-org/gitlab/-/issues/19367) its
requests, so only `authorization` is available for gitlab receivers,
while Github only supports request signature and not authorization
headers, so only `secret` for Github receivers. Gitea supports both
authorization and signature verification.

Most of the config values can be provided via ENV variables. Please consider
if it makes sense for your application to provide secrets in this manner.

### Supported git providers

| Provider | Can Authorize requests | Can Sign payload | Has Ping |
| -------- | ---------------------- | ---------------- | -------- |
| github   | false                  | true             | true     |
| gitea    | true                   | true[^1]         | false    |
| gitlab   | true                   | false            | false    |

Authorize means capability to provide Authorization header, which is then
verified by the service.

Sign payload is basically the same thing, but the whole payload is signed as a
measure to secure against payload MiM tampering.

Gitlab doesn't support payload signature, as per this [issue](https://gitlab.com/gitlab-org/gitlab/-/issues/19367)

[^1]:
    Can be insecure on plain http connections on gitea 1.14 or older because
    of [this issue](https://github.com/go-gitea/gitea/issues/11755)

### Project name restrictions

As project name is directly accessible in the url in `/projects/:proj_name` for
example, there are some restrictions applied on top of it, so it makes the url
easier to follow and we don't introduce any concerns. The project can contain
only letters and chars '\_', '-', '.', can't start or contain two or more
consecutive . in the name.

## Installation

### Build from source

To build the app, you need [go](https://go.dev/) version 1.26.2 or higher.
Since the app stores action outputs and logs in an SQLite3 database via
[go-sqlite3](https://github.com/mattn/go-sqlite3) you also need a `gcc`
compiler installed on your system and have `CGO_ENABLED=1` env variable set.

```sh
go install github.com/religiosa1/git-webhook-receiver@latest
```

Or you can clone the repo, and run the following command in its root dir:

```sh
go build
```

Please refer to the [docs/systemd-init-script.md](./docs/systemd-init-script.md)
for an example of a system.d script, that can be used to launch the
service at startup.

## SSL

It’s recommended to use SSL so that your requests are encrypted.
If you have an HTTP server such as Nginx or Caddy, you can use it to provide
a reverse proxy with SSL support.

Information on how to configure nginx + [certbot](https://certbot.eff.org/)
can be found [here](./docs/ssl-or-nginx-setup.md).

If you don’t have an HTTP server available, you can use the internal SSL
functionality by providing the certificate and key files in the corresponding
config fields.

## Inline scripts and standalone scripts

Inline scripts are processed using the [mdvan/sh](https://github.com/mvdan/sh)
interpreter to ensure they are cross-platform (in other words, they work on
Windows). These scripts are intended to be simple one- or multi-line bash-like
commands, such as "clone and run the build task."

Example:

```yaml
script: |
  git fetch && git reset --hard origin/master
  npm ci --ignore-scripts
  npm run build
```

If you need something more complicated, it's probably better to use `run` field
instead of `script` in the action config, passing a standalone script to it
(bash, Python, or any other language supported by your system). The `run` field
accepts its parameters in exec form, as an array of argv strings, in the
same way as docker's `CMD`
[does](https://docs.docker.com/reference/dockerfile/#exec-form):

Example:

```yaml
run: ["python", "./path/to/your/script", "--some-arg"]
```

In any case, you can optionally supply the `cwd` param, to specify the root dir
for execution and on unix-like systems `user` param,to specify the user who will
run the script.

Please notice, that `user` param is not supported on windows and your script
will always run from the same user, that launches the service.

`user` may be declared at three levels — config root,
project and action — forming a **root → project → action** hierarchy. A more
specific level overrides the one above it; an empty value inherits the parent.
This lets you set a single base user for every action and override it per
project or per action.

```yaml
user: deploy # base user for every action
projects:
  my_project:
    repo: "user/repo"
    user: www-data # overrides the root for this project's actions
    actions:
      - on: push
        user: root # overrides the project for this single action
        script: ./deploy.sh
```

### Environment supplied to actions

In both cases of `run` and `script` actions, the actual environment of the
service process is stripped from the environment supplied to actions, only
`PATH`, `HOME` and `USER` are left intact to avoid exposing secrets/auth from
env to actions.

Extra env variables are supplied to the action:

- `PROJECT_NAME` action's project name
- `ACTION_IDX` action index in config
- `PIPELINE_ID` unique pipeline id generated by the service
- `GIT_PROVIDER` project git provider, as specified in config
- `GIT_REPO` project git repo, as specified in config
- `DELIVERY_ID` delivery id, as supplied by git provider in webhook headers
- `GIT_COMMIT` git commit sha, as supplied by provider in the payload
- `GIT_BRANCH` git branch as supplied in the payload
- `GIT_EVENT` git event as supplied in the payload
- `CWD` the action's `cwd`, as specified in config (empty if unset)
- `TMPDIR` a managed temporary directory, only when `with_temp_dir` is set (see below)

#### Temporary directory

Set `with_temp_dir: true` on an action to have the service create a fresh
temporary directory before the action runs and expose its path as `$TMPDIR`.
The directory is removed once the action finishes — including on timeout or
cancellation, where an in-script `trap ... EXIT` would _not_ fire. This is the
recommended way to do clone-on-push style deploys without leaking the working
tree (which may contain credentials baked into the clone URL).

The directory is created with `0700` permissions. When the action runs as a
different `user`, ownership is handed to that user, so the contents stay
private to the single user the action runs as:

```yaml
actions:
  - on: push
    with_temp_dir: true
    cwd: "/var/www/app"
    environment:
      - GIT_TOKEN=${GIT_TOKEN:?must be set in the receiver env}
    script: |
      git clone --depth 1 https://${GIT_TOKEN}@github.com/${GIT_REPO} "$TMPDIR"
      cd "$TMPDIR"
      npm ci --ignore-scripts && npm run build
      cp -r dist/* "$CWD"
```

#### Custom environment variables

An action may declare an `environment` list of `KEY=VALUE` entries. They are
applied _on top_ of the built-in and passed-through variables above, so an entry
can override any of them (including `PATH`/`HOME`/`USER` or the `GIT_*` ones).

Each `VALUE` is subject to shell-style env interpolation, resolved against this
receiver process's environment (the same environment that is otherwise stripped
from actions) — this is the intended way to explicitly re-expose a selected
process variable to an action:

```yaml
actions:
  - on: push
    environment:
      - "DEPLOY_TOKEN=${DEPLOY_TOKEN:?must be set in the receiver env}"
      - "NODE_ENV=production"
      - "CACHE_DIR=${HOME}/.cache/myproject"
    script: ./deploy.sh
```

Supported operators (Docker-Compose-like): `${VAR}`, `${VAR:-default}`,
`${VAR-default}`, `${VAR:?err}`, `${VAR?err}`, `${VAR:+replacement}`,
`${VAR+replacement}`. Only parameter expansion is performed — command
substitution `$(...)` is rejected and globbing is disabled.

Variable substitution error check is happening during the action call time,
not at the start of the service.

##### Environment hierarchy

Like the `user` field, `environment` may be declared at three levels — config
root, project and action — forming a hierarchy that is applied **root → project
→ action**. Each level is layered on top of the previous one: a child level can
reference variables defined by its parents (and by preceding entries on the
same level), and, thanks to last-wins precedence, override them.

```yaml
environment:
  - "REGISTRY=registry.example.com"
projects:
  my_project:
    repo: "user/repo"
    environment:
      - "IMAGE=${REGISTRY}/${PROJECT_NAME}" # references root + built-in
    actions:
      - on: push
        environment:
          - "IMAGE=${IMAGE}:latest" # references, then overrides, the project value
        run: ["./deploy.sh"]
```

##### Masking

Environment entries may hold credentials, so — like `secret`/`authorization`
tokens — both their keys and values are **masked** everywhere they would
otherwise be shown: the config debug log, the inspection API and the Web UI. The
actual values are only ever passed to the action process itself.

## Inspection HTTP API

By default, the app exposes inspection HTTP endpoints, unless
`disable_api: true` is set in the config. These endpoints allows you to get the
status/output of a pipeline, list pipelines, or view the app logs.

```
GET /api/pipelines/{:pipeId} # To see the pipeline status
GET /api/pipelines/{:pipeId}/output # To see the pipe output
GET /api/pipelines # To list last pipelines
GET /api/logs # To see the logs result, must have logsdb on in config
```

You can find the full documentation for endpoints and params they accept
[here](./docs/inspection-api.md)

You can also enable BasicAuth for the API either in the config:

```
auth_password: "mysecret"
```

or with a env variable API_PASSWORD=mysecret

**Security Warning**: Do not use BasicAuth unless SSL is enabled (either in the
app or via a reverse proxy), as your credentials can sniffed.

## CLI

In addition to the default serve mode, the app provides a couple of CLI
subcommands to retrieve logs, inspect pipeline results, and retrieve their
output. It duplicates the HTTP-API functionality for the local access or cases
when HTTP-API is disabled.

Run `git-webhook-receiver --help` to see the list of available subcommands:

```
Usage: git-webhook-receiver <command> [flags]

Flags:
  -h, --help                  Show context-sensitive help.
  -c, --config-path=STRING    Configuration file name
  -v, --version               Show version information and exit

Commands:
  serve [flags]
    Run the webhook receiver server (default mode)

  pipeline (pl,get) [<pipeId>] [flags]
    Display pipeline record info

  output (cat) [<pipeId>] [flags]
    Display pipeline output

  list-pipelines (ls) [flags]
    Display a list of last N pipelines

  logs [flags]
    Display logs

Run "git-webhook-receiver <command> --help" for more information on a command.
```

Some examples:

You can use `pipeline` subcommand to check the last or given pipeline:

```sh
git-webhook-receiver pipeline # shows the last pipeline info
# OR
git-webhook-receiver pipeline <PIPE_ID>
```

## Logging

By default, action outputs are stored in a SQLITE database. Logs by default are
stored in SQLITE database and output to stdout to be captured by journalctl.
Name of db files are controlled by the `actions_db_file` and `logs_db_file`.

Actions' output is stored in the db once the action is completed.
While the action is still in progress, data is stored in a temporary file.

Setting those config values to an empty string will disable the persistent
storage of this information and in turn will also disable the corresponding
HTTP-API, admin UI pages and CLI subcommands.

Both databases use [Write-Ahead Logging](https://www.sqlite.org/wal.html).
This means, in addition to the file specified in the config, the app will also
create two additional temporary files during operation `<YOUR_FILE>-wal` and
`<YOUR_FILE>-shm`, to ensure data integrity during write operations.

Only N latest actions are stored in the directory, with N specified in the
config as `max_actions_stored` field. When number of output files exceeds this
number, the oldest actions (by their file LastModified date) are removed.
`max_actions_stored` defaults to 1000, setting it to a negative value turns
off this functionality.

## Contribution

If you have any ideas or suggestions or want to report a bug, feel free to
write in the issues section or create a PR.

### Testing the project locally without any git integration

If you want to quickly play around with the project, without actually doing
any kind of git integration, please refer to [this doc](./docs/local-run.md).

For local development with watch mode, this project uses [air](https://github.com/air-verse/air)
Install it, along with [templ](https://templ.guide/quick-start/installation)
and run `air`.

## License

git-webhook-receiver is MIT licensed.
