# Running the project locally without any git integration

If you want to quickly run the project locally without any git webhooks integration
you can clone the repo, create a test config file like this:

`config.yml`:

```yml
projects:
  test:
    git_provider: github
    repo: "username/reponame"
    actions:
      - on: push
        script: /usr/bin/env
        # run: ["sleep", "10"] # or run syntax:
```

Then you can run the project:

```sh
go run .
```

And try out sending a payload:

```http
POST http://localhost:9090/projects/test
X-GitHub-Event: push
X-GitHub-Delivery: blah
Content-Type: application/json

{
	"ref": "refs/heads/master",
	"after": "92bcfadb4199556415be69b9c31c0dc72343fea2",
  "repository": {
		"full_name": "username/reponame"
  }
}
```

Please note, that branch name in ref must match branch name in config (unless "\*" is used in the config)

See [captured requests](../internal/requestmock/captured-requests) folder, to see an actual
requests, to reuse for testing.
