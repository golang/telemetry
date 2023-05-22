# telemetrygodev

## Local Development

For local development, simply build and run. It serves on localhost:8080.

    go run .

## Testing

The telemetry.go.dev web site has a suite of regression tests that can be run with:

    go test golang.org/x/telemetry/...

## Deploying

Each time a CL is reviewed and submitted, the site is automatically deployed to Cloud Run.
If it passes its serving-readiness checks, it will be automatically promoted to handle traffic.

If the automatic deployment is not working, or to check on the status of a pending deployment,
see the “telemetrygodev” trigger in the
[Cloud Build history](https://pantheon.corp.google.com/cloud-build/builds?project=go-telemetry).

### Test Instance

To deploy a test instance of this service, push to a branch and manually trigger the deploy job from
the [Cloud Build console](https://pantheon.corp.google.com/cloud-build/triggers?project=go-telemetry)
with the desired values for branch and service name.
