// mrkvpn-control is an isolated, localhost-published alpha administration API.
// It manages desired inventory state; it does NOT control active WireGuard
// interfaces or distribute credentials to clients.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sysmrk/mrkvpn/control/internal/plane"
)

func main() {
	fs := flag.NewFlagSet("mrkvpn-control", flag.ExitOnError)
	listen := fs.String("listen", ":8787", "listen address (Docker publishes on host loopback only)")
	data := fs.String("data", "/data", "private persistent data directory")
	_ = fs.Parse(os.Args[1:])

	st, err := plane.OpenStore(*data)
	if err != nil {
		log.Fatalf("control store startup failed: %v", err)
	}
	defer st.Close()
	handler := plane.NewAPI(st).Handler()
	server := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       45 * time.Second,
		MaxHeaderBytes:    8192,
	}
	quit, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-quit.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	fmt.Println("mrkvpn control alpha listening. Keep port bound to host 127.0.0.1; no MFA/OIDC or gateway synchronization implemented.")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("control HTTP server: %v", err)
	}
}
