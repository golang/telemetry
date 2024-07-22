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

## Local Development

For local development, simply build and run. It serves on localhost:8082.

    go run ./godev/cmd/worker

By default, the server will use the filesystem for storage object I/O. Use the
-gcs flag to use the Cloud Storage API.

    go run ./godev/cmd/worker --gcs

Optionally, use the localstorage devtool the emulate the GCS server on your
machine.

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
