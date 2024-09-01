# git webhook receiver

Small service, that listens for the incoming webhhok HTTP-post from a git 
providers (gitea, github, gitlab, etc.) for one or many projects and runs 
a script/program on a matching webhook event.

In a nutshell:

- you [deploy](#installation) it to your server
- (optional but recommended) install an SSL cert or wrap it with
  a [reverse proxy](./docs/nginx-setup.md) such as nginx or caddy, to have 
  encryption
- create a [config file](#config-file) and add your projects and their 
  corresponding build scripts/actions (either as a standalone script 
  or [crossplatform inline scripts](#inline-scripts-and-standalone-scripts))
- set webhooks for those repo in their git services (github, gitea, etc.) to 
  post to {YOUR_HOST}/{PROJECT_NAME}
- start the service, it will listen for the webhook posts and runs the actions
  you described in the config (building your projects or whatever) when those 
  webhooks are fired from git

It's intended usage is to run CI/CD scripts on a server, but you can use it for
running arbitraty actions on git events.

WIP, but operational. Some planned MVP functionality is still missing and there
will be breaking changes before version 1.0 release.

## Config file

App requires a config file to start. By default, config file is read from
`./config.yml`, you can override this behavior by setting `CONFIG_PATH` env
variable or by using `--config` flag while launching the app (flag takes 
precedence).

A typical config file may look like this:

```yaml
host: example.com
ssl:
  cert_file_path: "./your/certfile/path/fullchain.pem"
  key_file_path: "/your/keyfile/path/privkey.pem"
projects:
  my_awesome_roject:
    repo: "username/reponame"
    secret: "YourSecretGoesHere"
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
see a list of all available configuration fields.

For security reason, it's recommended to always provide `secret` or at least
`authorization` (in case of gitea provider) param for every action. `secret` 
can also protect against MiM attacks, ensuring the payload wasn't tampered.

Most of the config values can be provided via ENV variables, please consider
if it makes sense for your application to provide secrets in this manner.

## Installation

<!-- ### TODO snap

Snap and flatpak package support is planned for 1.0 release. -->

### Build from source

To build the app, you'll require [go](https://go.dev/) version 1.22 or higher.
As the app stores its data in sqlite3 database via [go-sqlite3](https://github.com/mattn/go-sqlite3)
you also need a `gcc` compiler installed on your system and have `CGO_ENABLED=1`
env variable set.

```sh
go install github.com/religiosa1/git-webhook-receiver
```

Or you can clone the repo, and run the following command in its root dir:

```sh
go build
```

Please refer to the [docs/systemd-init-script.md](./docs/systemd-init-script.md)
for an example of system.d script, that can be used to launch the 
service at the startup.

## SSL

It's recomended to use SSL, so your requests are encrypted.
If you have a http-server such as Nginx or Caddy, you can leverage
it to provide you a reverse proxy with SSL support.

Infortmation on how to configure nginx + [certbot](https://certbot.eff.org/)
can be find [here](./docs/nginx-setup.md).

If you don't have an HTTP-server available, you can use internal
SSL functionality by passing cert and key files in the corresponding config
fields.

## Inline scripts and standalone scripts

Inline scripts are processed with [mdvan/sh](https://github.com/mvdan/sh) 
interpreter to ensure they're crossplatform (plainly speaking, working on 
windows). They're intended to be simple one or multiple lines scripts of 
bash-like code, like "clone and run the build task".

Example:

```yaml
script: |
  git fetch && git reset --hard origin/master
  npm ci
  npm run build
```

If you need something more complicated, it's probably better to use `run` field
instead of `script` in action config, allowing you to run a standalone script
(bash, python, whatever you like and whatever is supported by your system).
It accepts its parameters in exec form, as an array of argv strings, in the same
way as docker's `CMD` 
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

## Logging

By default, actions output is stored persistently in the sqlite database, while
application logs are not, as they're expected to be processed by the 
`journalctl` or another similar solution. Destination of actions output db file
is controlled by the `actions_db_file` param in the config, (`actions.sqlite3` 
by default) and actions will be stored there once completed. While the action 
is still going, data is stored in a temp file.

You can enable storage of application logs by providing `logs_db_file` in the 
config, so you can instpect them later (for example filtering by the delivery or
pipeline id). Please refer to the [schema file](./internal/logsDb/Init.sql), 
to see the list of available columns.

All of the logs are stored in sqlite db with 
[Write-Ahead Logging](https://www.sqlite.org/wal.html) turned on, which
means, that besides the file specified in the config, the app will also create
2 additional temporary files during the operation `<YOUR_FILE>-wal` and 
`<YOUR_FILE>-shm` to ensure data ingtegrity during the write operations.

You can use `pipeline` subcommand to check the last or given pipeline:

```sh
git-webhook-receiever pipeline
# OR
git-webhook-receiever pipeline <PIPE_ID>
```

Run `git-webhook-receiever pipeline --help` to see the list of all available
flags.

Run `get-webhook-receiver ls` to see a list of the last N pipelines. 

<!-- 
TODO implement this functionality for actionsDb:

Only N latest actions are stored in the directory, with N specified in the 
config as `max_output_files` field. When number of output files exceeds this 
number, the oldest actions (by their file LastModified date) are removed.
`max_output_files` defaults to 10000, setting it as 0 or negative value turns 
off this functionality. -->

## Contribution

If you have any ideas or suggestions or want to report a bug, feel free to
write in the issues section or create a PR.

## License

git-webhook-reciever is MIT licensed.
