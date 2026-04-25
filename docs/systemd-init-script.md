# Systemd init script example

If you're deploying application manually after compilation, you'll need
a startup script for the service.

On a linux system with systemd (basically all of them), it will be a systemd
unit, which you can install like this:

```sh
sudo systemctl edit --full --force git-webhook-receiver.service
# or edit `/etc/systemd/system/git-webhook-receiver.service`
```

```
[Unit]
Description=Git webhook receiver
Documentation=https://github.com/religiosa1/git-webhook-receiver
# Or whatever is needed in your distro, see nginx unit file
After=network-online.target

[Service]
# the path where you installed your script
ExecStart=/usr/local/bin/git-webhook-receiver -c /etc/git-webhook-receiver.yaml
Restart=on-failure
# Optional, e.g. for storing DB there
WorkingDirectory=/var/data/git-webhook-receiver

# Extra security stuff, just to make sure your actions won't mess anything up:

# setting user/group to www-data, assuming that's what your httpd uses and you
# want to serve the result
User=www-data
Group=www-data

PrivateTmp=yes # to isolate tmp, so actions don't see your regular /tmp
ProtectSystem=strict # file system access is readonly
# explicitely give access to the required folders (adjust to your needs)
ReadWritePaths=/var/www /var/run +/

[Install]
WantedBy=multi-user.target
```

After adding this file, you can start the service with:

```sh
sudo systemctl daemon-reload # reload configuration
sudo systemctl start git-webhook-receiver # start the service
sudo systemctl enable git-webhook-receiver # enable automatic start on startup
```

To access logs afterwards:

```sh
journalctl -u git-webhook-receiver.service
```

Consult to systemd documentation for more.

## Unix socket setup

To listen on a Unix socket instead of a TCP port, set `addr` in your config:

```yaml
addr: unix:///var/run/git-webhook-receiver.sock
```

The recommended approach is to place the socket in a directory owned by the
group shared between this service and your reverse proxy (e.g. `www-data`),
with mode `0750`:

```sh
sudo mkdir -p /var/run/git-webhook-receiver
sudo chown git-webhook-receiver:www-data /var/run/git-webhook-receiver
sudo chmod 750 /var/run/git-webhook-receiver
```

Then point the config at a socket inside that directory:

```yaml
addr: unix:///var/run/git-webhook-receiver/webhook.sock
```

Update the systemd unit to give the service write access to that directory and
set a `UMask` so the socket is created with `0660`:

```
[Service]
User=git-webhook-receiver
Group=git-webhook-receiver
UMask=0117
ReadWritePaths=/var/www /var/run/git-webhook-receiver
```

Your reverse proxy (nginx, caddy, etc.) must run as a user in the
`git-webhook-receiver` group, or the directory must be world-executable (`0751`)
so the proxy can reach the socket.

### To change your systemd editor:

- add `export SYSTEMD_EDITOR=vim` to your `~/.bashrc` (vim for example)
- reload it `source ~/.bashrc`
- ensure your variable is forwarded to systemd in sudo: run `sudo visudo` find
  some lines containing default and add `Defaults env_keep += "SYSTEMD_EDITOR"`
