package serve

import (
	"fmt"
	"net/http"
	"rancherlabs/cattle-drive/pkg/api"

	"github.com/urfave/cli/v2"
)

var listenAddr string

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
		},
		Action: serve,
	}
}

func serve(clx *cli.Context) error {
	handler := api.NewServer()
	fmt.Printf("cattle-drive API server listening on %s\n", listenAddr)
	fmt.Println("Endpoints:")
	fmt.Println("  POST /api/clusters  - list downstream clusters")
	fmt.Println("  POST /api/status    - get migration status")
	fmt.Println("  POST /api/migrate   - run migration")
	fmt.Println("  GET  /healthz       - health check")
	return http.ListenAndServe(listenAddr, handler)
}
