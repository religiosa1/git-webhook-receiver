# Systemd init script example

If you're depploying application manually after compilation, you'll need
a startup script for the service.

On systems with systemd startup you can use
a script like that:

`/etc/systemd/system/git-webhook-receiver.service`:

```
[Unit]
Description=Git webhook receiver startup script

[Service]
# path where your db files will be stored
WorkingDirectory=/var/www/deploy
# path to the binary with optional path to config
ExecStart=/root/go/bin/git-webhook-receiver -c /root/receiver-config.yaml

[Install]
WantedBy=multi-user.target
```

After adding this file, you can start the service with:

```sh
systemctl start git-webhook-receiver.service
```

Or access its logs like this:

```sh
journalctl -u git-webhook-receiver.service
```
