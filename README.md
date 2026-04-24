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
# provide it through env API_PASSWORD, if you don't want it readable in config
api_password: "password for inspection api basic auth"
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
          npm ci
          npm run build
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
  npm ci
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

## Inspection HTTP API

By default, the app exposes inspection HTTP endpoints, unless
`disable_api: true` is set in the config. These endpoints allows you to get the
status/output of a pipeline, list pipelines, or view the app logs.

```
GET /pipelines/{:pipeId} # To see the pipeline status
GET /pipelines/{:pipeId}/output # To see the pipe output
GET /pipelines # To list last pipelines
GET /logs # To see the logs result, must have logsdb on in config
```

You can find the full documentation for endpoints and params they accept
[here](./docs/inspection-api.md)

You can also enable BasicAuth for the APi either in the config:

```
api_password: "mysecret"
```

or with a env variable API_PASSWORD=mysecret

**Security Warning**: Do not use BasicAuth unless SSL is enabled (either in the
app or via a reverse proxy), as your credentials can sniffed.

## CLI

In addition to the default serve mode, the app provides a couple of CLI
subcommands to retrieve logs, inspect pipeline results, and retrieve their
output. It duplicates the HTTP-API functionality for the local access or cases
when HTTP-API is disabled.

Run `git-webhook-receiver --help` to see the list of available subcommands
or run `git-webhook-receiver <SUBCOMMAND> --help` to subcommand help.

Some examples:

You can use `pipeline` subcommand to check the last or given pipeline:

```sh
git-webhook-receiver pipeline # shows the last pipeline
# OR
git-webhook-receiver pipeline <PIPE_ID>
```

Run `get-webhook-receiver ls` to see a list of the last N pipelines.
Run `get-webhook-receiver logs` to inspect app logs.

## Logging

By default, action outputs are stored in a SQLITE database. Logs by default are
only output to stdout to be captured by journalctl, but they can also be stored
persistently in a separate db, in case you want to expose them through an
endpoint. This is controlled with the `logs_db_file` config option.

By default, actions db filename is `actions.sqlite3`. This filenames are
controlled by the `actions_db_file`.

Actions' output is stored in the db once the action is completed.
While the action is still in progress, data is stored in a temporary file.

Setting those config values to an empty string will disable the persistent on
storage of this information and in turn will also disable the corresponding
HTTP-API and/or CLI subcommands.

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

## License

git-webhook-receiver is MIT licensed.
