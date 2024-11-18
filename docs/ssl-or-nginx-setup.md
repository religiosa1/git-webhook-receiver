## Nginx configuration

TODO

Example nginx configuration file, prior to launching certbot:

```
server {
	server_name your.server.name.com;
	listen 80;
	listen [::]:80;

	gzip on;
	gzip_types text/plain text/css application/javascript application/json application/xml;

	location / {
		proxy_pass http://127.0.0.1:9090;
	}
}
```
