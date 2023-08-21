# telemetrygodev

## Local Development

For local development, simply build and run. It serves on localhost:8080.

    go run ./cmd/telemetrygodev

By default, the server will use the filesystem for storage object I/O. Run the
cloud storage emulator and use the -gcs flag to use the Cloud Storage API.

    ./devtools/localstorage.sh
    go run ./cmd/telemetrygodev --gcs

### Environment Variables

| Name                               | Default               | Description                                               |
| ---------------------------------- | --------------------- | --------------------------------------------------------- |
| GO_TELEMETRY_PROJECT_ID            | go-telemetry          | GCP project ID                                            |
| GO_TELEMETRY_STORAGE_EMULATOR_HOST | localhost:8081        | Host for the Cloud Storage emulator                       |
| GO_TELEMETRY_LOCAL_STORAGE         | .localstorage         | Directory for storage emulator I/O or file system storage |
| GO_TELEMETRY_UPLOAD_CONFIG         | ../config/config.json | Location of the upload config used for report validation  |
| GO_TELEMETRY_MAX_REQUEST_BYTES     | 102400                | Maximum request body size the server allows               |
| GO_TELEMETRY_ENV                   | local                 | Deployment environment (e.g. prod, dev, local, ... )      |

## Testing

The telemetry.go.dev web site has a suite of regression tests that can be run
with:

    go test golang.org/x/telemetry/...

## Deploying

Each time a CL is reviewed and submitted, the site is automatically deployed to
Cloud Run. If it passes its serving-readiness checks, it will be automatically
promoted to handle traffic.

If the automatic deployment is not working, or to check on the status of a
pending deployment, see the “telemetrygodev” trigger in the
[Cloud Build history](https://pantheon.corp.google.com/cloud-build/builds?project=go-telemetry).

### Test Instance

To deploy a test instance of this service, push to a branch and manually trigger
the deploy job from the
[Cloud Build console](https://pantheon.corp.google.com/cloud-build/triggers?project=go-telemetry)
with the desired values for branch and service name.
