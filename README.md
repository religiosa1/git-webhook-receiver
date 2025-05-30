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
  Nginx or Caddy, to enable encryption.
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
host: example.com
ssl:
  cert_file_path: "/etc/letsencrypt/live/example.com/fullchain.pem"
  key_file_path: "/etc/letsencrypt/live/example.com/privkey.pem"
api_password: "password for inspection api basic auth"
projects:
  my_awesome_roject:
    repo: "username/reponame"
    secret: "YourSecretGoesHere" # generate it with `openssl rand -base64 42`
    actions:
      - on: push
        branch: main
        user: www-data
        cwd: "/var/www/default"
        script: |
          git fetch && git reset --hard origin/main
          npm ci
          npm run build
  my_other_project:
    repo: "username/reponame2"
    secret: "YourSecretGoesHere"
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
requests, so only `authorization` is availabe for gitlab receivers,
while Github only supports request signature and not authorization
headers, so only `secret` for Github receivers. Gitea supports both
authorization and signature verification.

Most of the config values can be provided via ENV variables. Please consider
if it makes sense for your application to provide secrets in this manner.


## Installation

<!-- ### TODO snap

Snap and flatpak package support is planned for 1.0 release. -->

### Build from source

To build the app, you need [go](https://go.dev/) version 1.22 or higher.
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

Infortmation on how to configure nginx + [certbot](https://certbot.eff.org/)
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
GET /logs # To see the logs result
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

Run `git-webhook-receiver --help` to see the list of available subcomands 
or run `git-webhook-receiver <SUBCOMMAND> --help` to subcommand's help.

Some examples:

You can use `pipeline` subcommand to check the last or given pipeline:

```sh
git-webhook-receiever pipeline # shows the last pipeline
# OR
git-webhook-receiever pipeline <PIPE_ID>
```

Run `get-webhook-receiver ls` to see a list of the last N pipelines.
Run `get-webhook-receiver logs` to inspect app logs.

<!--
TODO implement this functionality for actionsDb:

Only N latest actions are stored in the directory, with N specified in the
config as `max_output_files` field. When number of output files exceeds this
number, the oldest actions (by their file LastModified date) are removed.
`max_output_files` defaults to 10000, setting it as 0 or negative value turns
off this functionality. -->

## Logging

By default, action outputs and logs are stored persistently in two SQLite 
databases: one for app logs and one for actions and their outputt.
By default, actions db filename is `actions.sqlite3`, logs db filename is 
`logs.sqlite3`. This filenames are controlled by the `actions_db_file` and 
`logs_db_file` fields in the config correspondingly. 

Setting those config values to an empty string will disable the persistent on 
storage of this information and in turn will also disable the corresponding 
HTTP-API and/or CLI subcommands.

Actions' output is stored in the db once the action is completed.
While the action is still in progress, data is stored in a temporary file.

Both databases use [Write-Ahead Logging](https://www.sqlite.org/wal.html).
This means, in addition to the file specified in the config, the app will also 
create two additional temporary files during operation `<YOUR_FILE>-wal` and
`<YOUR_FILE>-shm`, to ensure data ingtegrity during write operations.

## Contribution

If you have any ideas or suggestions or want to report a bug, feel free to
write in the issues section or create a PR.

## License

git-webhook-reciever is MIT licensed.
