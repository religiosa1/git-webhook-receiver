# git webhook-receiver

Small service, that's listening for the incoming HTTP-post from a git providers
(gitea, github, gitlab, etc.) for multiple possible projects and runs
a script/program on a matching webhook event.

In a nutshell:

- you [deploy](#installation) it to your server
- (optional but recommended) install an SSL cert or wrap it with
  a [reverse proxy](#reverse-proxy) such as nginx or caddy, to have encryption
- create a [config file](#config-file) and add your projects and their 
  corresponding build scripts/actions (either as a standalone script 
  or [crossplatform inline scripts](#inline-scripts))
- set webhooks for those repo in their git services (github, gitea, etc.) to post
  to {YOUR_HOST}/{PROJECT_NAME}
- start the service, it will listen for the webhook posts and runs the actions
  you described in the config (building your projects or whatever) when those 
  webhooks are fired from git

It's intended usage is to run CI/CD scripts on a server, but you can use it for
running arbitraty actions on git events.

Early WIP, currently only gitea is supported as a git provider.

## Installation

<!-- ### TODO snap

Snap and flatpak package support is planned for 1.0 release. -->

### Build from source

To build the app, you'll require [go](https://go.dev/) version 1.22 or higher.

```sh
go install github.com/religiosa1/webhook-receiver
```

Or you can clone the repo, and run the following command in its root dir:

```sh
go build
```

Please refer to the [docs/systemd-init-script.md](./docs/systemd-init-script.md)
for an example of system.d script, that can be used to launch the 
service at the startup.

## Config file

App requires a config file to start. By default, config file is read from
`./config.yml`, you can override this behavior by setting `CONFIG_PATH` env
variable or by using `--config` flag while launching the app (flag takes 
precedence).

Please refer to the [config file example](./config.example.yml) in this repo, to
see a list of all available configuration fields.

For security reason, it's recommended to always provide `secret` or at least
`authorization` param for every action. `secret` can also protect against 
MiM attacks, ensuring the payload wasn't tampered.


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

```
script: |
  git fetch && git reset --hard origin/master
  npm ci
  npm run build
```

If you need something more complicated, it's probably better to use `run` field
instead of `script` in action config, allowing you to run a standalone script
(bash, python, whatever you like and whatever is supported by your system).
It accepts its parameters in exec form, as an array of argv strings, in the same
way as docker's `CMD` [does](https://docs.docker.com/reference/dockerfile/#exec-form):

Example:

```
run: ["python", "./path/to/your/script", "--some-arg"]
```

## Logging

Actions' output is stored in the directory specified in the config file.
If no directory is specified, the default value of `./actions_output` will be used.

If the directory is explicitely set to be empty, then storing of actions' output
will be disabled.

Only N latest actions are stored in the directory, with N specified in the config
as `max_output_files` field. When number of output files exceeds this number,
the oldest actions (by their file LastModified date) are removed.
`max_output_files` defaults to 10000, setting it as 0 or negative value turns off
this functionality.

## Contribution

If you have any ideas or suggestions or want to report a bug, feel free to
write in the issues section or create a PR.

## License

webhook-reciever is MIT licensed.
