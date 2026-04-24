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

### To change your systemd editor:

- add `export SYSTEMD_EDITOR=vim` to your `~/.bashrc` (vim for example)
- reload it `source ~/.bashrc`
- ensure your variable is forwarded to systemd in sudo: run `sudo visudo` find
  some lines containing default and add `Defaults env_keep += "SYSTEMD_EDITOR"`
