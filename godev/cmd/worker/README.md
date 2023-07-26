# worker

## Endpoints

### `/merge/?date=<YYYY-MM-DD>`

The merge endpoint reads the set of reports from the upload bucket prefixed with
the value of the data param and encodes each report as newline separated JSON in
a merged report. It returns the number of reports merged and the location of the
merged report.

## Local Development

For local development, simply build and run. It serves on localhost:8082.

    go run ./cmd/worker

By default, the server will use the filesystem for storage object I/O. Run the
cloud storage emulator and use the -gcs flag to use the Cloud Storage API.

    ./devtools/localstorage.sh
    go run ./cmd/worker --gcs

### Environment Variables

| Name                               | Default               | Description                                               |
| ---------------------------------- | --------------------- | --------------------------------------------------------- |
| GO_TELEMETRY_PROJECT_ID            | go-telemetry          | GCP project ID                                            |
| GO_TELEMETRY_STORAGE_EMULATOR_HOST | localhost:8081        | Host for the Cloud Storage emulator                       |
| GO_TELEMETRY_LOCAL_STORAGE         | .localstorage         | Directory for storage emulator I/O or file system storage |
| GO_TELEMETRY_UPLOAD_CONFIG         | ../config/config.json | Location of the upload config used for report validation  |
| GO_TELEMETRY_MAX_REQUEST_BYTES     | 102400                | Maximum request body size the server allows               |

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
