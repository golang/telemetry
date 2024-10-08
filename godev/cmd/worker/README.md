# worker

## Endpoints

### `/merge/?date=<YYYY-MM-DD>`

The merge endpoint reads the set of reports from the upload bucket prefixed with
the value of the data param and encodes each report as newline separated JSON in
a merged report. It returns the number of reports merged and the location of the
merged report.

### `/chart`

The /chart endpoint reads the file named 'YYYY-MM-DD.json' containing reports
from the given date from the merge bucket and generates chart data based on
these reports. It returns the number of reports used to generate the chart and
the location where the chart data is stored.

#### `/chart/?date=<YYYY-MM-DD>`

Use this endpoint to generate charts from a report on a specific date. The
worker will retrieve the report for the provided date from the merge bucket.

#### `/chart/?start=<YYYY-MM-DD>&end=<YYYY-MM-DD>`

Use this endpoint to generate an aggregate chart file containing data from the
provided date range (inclusive) from the merge bucket.

### `/copy` (dev env only)

This endpoint facilitates the copying of uploaded reports from the prod
environment's "uploaded" bucket to the dev environment's "uploaded" bucket.

Since we don't have clients regularly uploading data to dev, copying seeds the
dev environment with data.

Similar to the /chart endpoint, /copy also supports the following query
parameters:

- `/copy/?date=<YYYY-MM-DD>`: Copies reports for a specific date.
- `/copy/?start=<YYYY-MM-DD>&end=<YYYY-MM-DD>`: Copies reports within a
  specified date range.

### `/queue-tasks`

The queue-tasks endpoint is responsible for task distribution. When invoked, it
triggers the following actions:

- call merge endpoint to merge uploaded reports for the past 7 days.
- call chart endpoint to generate daily charts for the 7 days preceding today.
- call chart endpoint to generate weekly charts for the past 8 days.

## Local Development

The preferred method of local develoment is to simply build and run the worker
binary. Use PORT= to customize the default hosting port.

    go run ./godev/cmd/worker

By default, the server will use the filesystem for storage object I/O (see
[`GO_TELEMETRY_LOCAL_STORAGE`](#environment-variables)). Unless you have also
uploaded reports through a local instance of the telemetry frontend, this local
storage will be empty. To copy uploads from GCS to the local environment, run:

    go run ./godev/devtools/cmd/copyuploads -v

Note that this command requires read permission to our GCS buckets.

So, this is a complete end-to-end test of the merge endpoint:

1. First, copy data with:

   ```
   go run ./godev/devtools/cmd/copyuploads -v
   ```

2. Then, run the worker:

   ```
   go run ./godev/cmd/worker
   ```

3. Finally, in a separate terminal, trigger the merge operation:

   ```
   curl http://localhost:8082/merge/?date=2024-09-26
   ```

After doing this, you should see the resulting merged reports in the
`./localstorage/local-telemetry-merged` directory.

Note: the `/queue-tasks/` endpoint does not currently work locally: by default
it tries to enqueue tasks in the associated GCP project, which will fail unless
you have escalated permissions on GCP.

### Local development using GCS

Alternatively, you can use the -gcs flag to use the Cloud Storage API:

    go run ./godev/cmd/worker --gcs

However, the above command requires write permissions to our public GCS buckets,
which one should in general not request. Instead, use the localstorage devtool
the emulate the GCS server on your machine.

    ./godev/devtools/localstorage.sh
    STORAGE_EMULATOR_HOST=localhost:8081 go run ./godev/cmd/worker --gcs

### Environment Variables

| Name                           | Default               | Description                                               |
| ------------------------------ | --------------------- | --------------------------------------------------------- |
| GO_TELEMETRY_PROJECT_ID        | go-telemetry          | GCP project ID                                            |
| GO_TELEMETRY_LOCAL_STORAGE     | .localstorage         | Directory for storage emulator I/O or file system storage |
| GO_TELEMETRY_UPLOAD_CONFIG     | ../config/config.json | Location of the upload config used for report validation  |
| GO_TELEMETRY_MAX_REQUEST_BYTES | 102400                | Maximum request body size the server allows               |
| GO_TELEMETRY_ENV               | local                 | Deployment environment (e.g. prod, dev, local, ... )      |
| GO_TELEMETRY_LOCATION_ID       |                       | GCP location of the service (e.g, us-east1)               |
| GO_TELEMETRY_SERVICE_ACCOUNT   |                       | GCP service account used for queueing work tasks          |
| GO_TELEMETRY_CLIENT_ID         |                       | GCP OAuth client used in authentication for queue tasks   |
| GO_TELEMETRY_WORKER_URL        | http://localhost:8082 |                                                           |

## Testing

The worker servie has a suite of regression tests that can be run with:

    go test golang.org/x/telemetry/...

## Deploying

Each time a CL is reviewed and submitted, the site is automatically deployed to
Cloud Run. If it passes its serving-readiness checks, it will be automatically
promoted to handle traffic.

If the automatic deployment is not working, or to check on the status of a
pending deployment, see the “worker” trigger in the
[Cloud Build history](https://console.cloud.google.com/cloud-build/builds?project=go-telemetry).

### Test Instance

To deploy a test instance of this service, push to a branch and manually trigger
the deploy job from the
[Cloud Build console](https://console.cloud.google.com/cloud-build/triggers?project=go-telemetry)
with the desired values for branch and service name.
