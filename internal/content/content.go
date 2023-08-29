package content

import (
	"embed"
	"log"
	"os"
	"os/exec"
)

//go:embed *
var FS embed.FS

//go:generate go run golang.org/x/telemetry/godev/devtools/cmd/esbuild --outdir gotelemetryview/static gotelemetryview
//go:generate go run golang.org/x/telemetry/godev/devtools/cmd/esbuild --outdir shared/static shared
//go:generate go run golang.org/x/telemetry/godev/devtools/cmd/esbuild --outdir telemetrygodev/static telemetrygodev

// watchStatic runs the same command as the generator above when the server is
// started in dev mode, rebuilding static assets on save.
func WatchStatic() {
	for _, dir := range []string{"gotelemetryview", "shared", "telemetrygodev"} {
		cmd := exec.Command("go", "run", "golang.org/x/telemetry/godev/devtools/cmd/esbuild", "--outdir", dir+"/static", "--watch", dir)
		cmd.Dir = "internal/content"
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		go func() {
			if err := cmd.Wait(); err != nil {
				log.Fatal(err)
			}
		}()
	}
}
