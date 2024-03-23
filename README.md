# git webhook-receiver

Small service, that's listening for the incoming HTTP-post from a git providers
(gitea, github, gitlab, etc.) for multiple possible projects and runs
a script/program on a matching webhook event.

In a nutshell:

- you [deploy](#installation) it to your server
- (optional but recommended) wrap it with a [reverse proxy](#reverse-proxy),
  such as nginx or caddy, to have SSL
- add your git repos and corresponding build scripts/actions in the
  [config file](#config-file)
- set webhooks for those repo in their git services (github, gitea, etc.)
- start the service and now it runs the scripts (building your projects or
  whatever) when those webhooks are fired from git

It's intended usage is to run CI/CD scripts on a server, but you can use it for
running arbitraty actions on git events.

Early WIP, currently only gitea is supported as a git provider, more incoming.

## Installation

### TODO snap

Snap package support is planned for 1.0 release.

### Build from source

To build the app, you'll require [go](https://go.dev/) version 1.22 or higher.

```sh
go install github.com/religiosa1/webhook-receiver
```

Or you can clone the repo, and run the following command in its root dir:

```sh
go build
```

## Reverse proxy

It's recomended to use some kind of reverse proxy and SSL, to provide
encryption.

Example of nginx + [certbot](https://certbot.eff.org/) configuration can be find
[here](./docs/nginx-setup.md).

## Config file

App requires a config file to start. By default, config file is read from
`./config.yml`, you can override this behavior by setting `CONFIG_PATH` env
variable or by using `--config` flag while launching the app (flag takes precedence).

TODO config file format description.

## Contribution

If you have any ideas or suggestions or want to report a bug, feel free to
write in the issues section or create a PR.

## License

webhook-reciever is MIT licensed.
