# Systemd init script example

If you're depploying application manually after compilation, you'll need
a startup script for the service.

On systems with systemd startup you can use
a script like that (assuming you cloned and compiled the app in `/var/www/deploy` directory).

`/etc/systemd/system/git-webhook-receiver.service`:

```
[Unit]
Description=Git webhook receiver startup script

[Service]
WorkingDirectory=/var/www/deploy
ExecStart=/var/www/deploy/git-webhook-receiver

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
