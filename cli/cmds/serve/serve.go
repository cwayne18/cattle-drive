package serve

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"rancherlabs/cattle-drive/pkg/api"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

var (
	listenAddr string
	apiToken   string
)

// NewCommand returns the "serve" CLI sub-command that starts the HTTP API server.
func NewCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Start the cattle-drive HTTP API server for the Rancher UI extension",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "listen",
				Usage:       "Address and port to listen on",
				Value:       "0.0.0.0:8080",
				Destination: &listenAddr,
			},
			&cli.StringFlag{
				Name:        "api-token",
				Usage:       "Bearer token required by all API requests (recommended in production)",
				EnvVars:     []string{"CATTLE_DRIVE_API_TOKEN"},
				Destination: &apiToken,
			},
		},
		Action: serve,
	}
}

func serve(clx *cli.Context) error {
	opts := api.ServerOptions{APIToken: apiToken}
	srv := &http.Server{
		Addr:    listenAddr,
		Handler: api.NewServer(opts),
	}

	fmt.Printf("cattle-drive API server listening on %s\n", listenAddr)
	if apiToken != "" {
		fmt.Println("Authorization: Bearer token required for all API requests")
	} else {
		fmt.Println("Warning: no --api-token set; the API is open to anyone with network access")
	}
	fmt.Println("Endpoints:")
	fmt.Println("  POST /api/clusters  - list downstream clusters")
	fmt.Println("  POST /api/status    - get migration status")
	fmt.Println("  POST /api/migrate   - run migration")
	fmt.Println("  GET  /healthz       - health check")

	// Start the server in a goroutine so we can wait for a shutdown signal.
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Block until SIGINT or SIGTERM, then perform a graceful 10-second shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		fmt.Printf("\nReceived signal %s – shutting down…\n", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
