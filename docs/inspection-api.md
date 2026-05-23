# Inspection HTTP API

Unless `disable_api: true` is set in your configuration, the app will create 
several additional endpoints, which will allow you to retrieve information
about pipelines and logs in JSON format.

You can optionally set HTTP BasicAuth on those endpoints, using the following
configuration fields:

```
auth_user: "admin" # Basic auth user for the inspection API and Web UI.
auth_password: "" # Basic auth password for the inspection API and Web UI. Empty string disables basicauth
```

**Security Warning**: Do not use BasicAuth unless SSL is enabled (either in the
app or via a reverse proxy), as your credentials can sniffed.

Please note that this functionality requires persistent storage for logs and
action data. Ensure that the `logs_db_file` and `actions_db_file` fields in
the configuration are not left empty.

## Available endpoints:

### GET /api/pipelines

Returns a list of last N run pipelines.

N is 20 by default.

#### Example:

```http
GET /api/pipelines
```

##### RESPONSE:
```json
{
  "items": [
    {
      "pipeId": "01J877WS50T0P6CT50PX408CS9",
      "project": "your_project_name",
      "deliveryId": "test",
      "config": {
        "branch": "master",
        "on": "push",
        "run": [
          "sleep",
          "10"
        ]
      },
      "error": null,
      "createdAt": "2024-09-20T10:13:37+02:00",
      "endedAt": "2024-09-20T10:13:47+02:00"
    },
    {
      "pipeId": "01J8DCJS1K10N1CTEB2T30E4RT",
      "project": "your_project_name",
      "deliveryId": "test",
      "config": {
        "branch": "master",
        "on": "push",
        "script": "env",
        "user": "www-data"
      },
      "error": "exit status 127",
      "createdAt": "2024-09-22T19:30:58+02:00",
      "endedAt": "2024-09-22T19:30:58+02:00"
    },
  ],
  "totalCount": 32167,
  "nextPage": "/api/pipelines?cursor=1778783462054_7"
}
```

`nextPage` will contain relative URL if no publicURL is provided in config, 
otherwise it will be an absolute URL.

#### Available query params:

- `offset`: `int`
  The number of pipelines to skip before starting to return results. Can be for pagination.
- `cursor`: `string`
  For usage with cursor pagination, as returned in `nextPage` the response 
  field. Cursor and offset pagination can't be supplied simultaneously.
- `cursor`: `string`
  For usage with cursor pagination, as returned in `nextPage` response field. Cursor and offset 
  pagination can't be supplied simultaneously.
- `project`: `string`
  Filters the pipelines by project name.
- `deliveryId`: `string`
  Filters the pipelines by deliveryId.
- `status`: "`ok" | "error" | "pending" | "any"`
  Filters pipelines based on their completion status:
  - "ok": Only returns pipelines that completed successfully.
  - "error": Only returns pipelines that encountered an error.
  - "pending": Returns pipelines that are still in progress or haven't finished yet.
  - "any": Returns pipelines regardless of their status (default behavior if no status is specified).

### GET /api/pipelines/{pipeId}

Get pipeline basic info.

#### Example:

```http
GET /api/pipelines/01J8DCJS1K10N1CTEB2T30E4RT
```

##### RESPONSE:
```json
{
  "pipeId": "01J8DCJS1K10N1CTEB2T30E4RT",
  "project": "your_project_name",
  "deliveryId": "2b2bb38c-5e23-48d9-934b-882ed82c5276",
  "config": {
    "branch": "master",
    "on": "push",
    "run": [
      "sleep",
      "10"
    ]
  },
  "error": null,
  "createdAt": "2024-09-20T10:13:37+02:00",
  "endedAt": "2024-09-20T10:13:47+02:00"
}
```

Output is the same as in list format, but only returns a single record.
If the pipeline is still pending, `endedAt` will be null, otherwise it will
contain the ending datetime of the operation.

`error` will contain error message, if the pipeline ended with error.

### GET /api/pipelines/{pipeId}/output

Returns pipeline output.

#### Example:

```http
GET /api/pipelines/01J8DCJS1K10N1CTEB2T30E4RT/output
```

##### RESPONSE:
```
The actual pipeline output.
```

Returns the recorded output of the pipeline for the ended pipelines.
If the pipeline is still pending it will return an empty response.

Response's content-type is always `text/plain`, containing cumulative output
from both STDOUT and STDERR of the pipeline.

### GET /api/logs

Returns a list of last N app log entries.

N is 20 by default.

#### Example:

```http
GET /api/logs
```

##### RESPONSE:
```json
{
  "items": [
    {
      "level": "debug",
      "project": null,
      "deliveryId": null,
      "pipeId": null,
      "message": "Registered project",
      "data": {
        "projectName": "your_project_name",
        "repo": "username/reponame",
        "type": "gitea"
      },
      "ts": "2024-09-24T01:24:46+02:00"
    },
    {
      "level": "info",
      "project": null,
      "deliveryId": null,
      "pipeId": null,
      "message": "Server closed",
      "data": null,
      "ts": "2024-09-24T01:24:51+02:00"
    }
  ],
  "totalCount": 504,
  "nextPage": "/api/logs?cursor=1779451301799_485"
}
```

`nextPage` will contain relative URL if no publicURL is provided in config, 
otherwise it will be an absolute URL.

#### Available query params:

- `offset`: `int`
  The number of log entries to skip before starting to return results.
- `limit`: `int`
  The maximum number of log entries to return. Defaults to 20, min is 0 max is 1000.
- `cursor`: `string`
  For usage with cursor pagination, as returned in `nextPage` the response 
  field. Cursor and offset pagination can't be supplied simultaneously.
- `level`: `Array<"debug" | "info" | "warn" | "error">`
  Filters logs by severity level. This query parameter can be repeated multiple
  times (e.g., ?level=warn&level=error) to include multiple log levels in the
  response. Defaults to all available levels.
- `project`: `string`
  Filters log entries by project name.
- `deliverId`: `string`
  Filters logs by deliveryId value.
- `pipeId`: `string`
  Filters log entries by pipeId.
- `message`: `string`
  Filters logs that contain a specific string in the message field

