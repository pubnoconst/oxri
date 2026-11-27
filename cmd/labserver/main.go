package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/oxri/oxri-go/internal/labsite"
)

// Command labserver serves the site on loopback from an in-memory database
// seeded with entries like real ones, for a lab that runs in another process
// or on another machine (cmd/lab/lab.mjs -local; CI). What it holds is in
// internal/labsite.
//
//	go run ./cmd/labserver                              # 14 entries, a port chosen by the system
//	go run ./cmd/labserver -seed 160 -page-size 24 -listen 127.0.0.1:8090
//
// Its first line of output is the address it serves on, for the lab to read.
// It stops on Ctrl-C, or when whoever started it closes its input.
func main() {
	listen := flag.String("listen", "127.0.0.1:0", "address to serve on (port 0: one chosen by the system)")
	seed := flag.Int("seed", 14, "how many entries to seed (every other one a tweet clipping with a photo)")
	pageSize := flag.Int("page-size", 0, "entries per catalog page, to compare sizes (0 = the site's own)")
	flag.Parse()

	site, err := labsite.New(context.Background(), *seed, *pageSize)
	if err != nil {
		fail("seed the site: %v", err)
	}
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fail("listen on %s: %v", *listen, err)
	}
	srv := &http.Server{Handler: site.Handler(), ReadHeaderTimeout: 5 * time.Second}

	// The address, as the first line, for the lab that started this.
	fmt.Println("http://" + ln.Addr().String())

	var once sync.Once
	// Shutdown waits for the requests in flight (the one that asked, if that
	// is how we were told) before it closes the listener, so it runs apart
	// from them.
	stop := func() {
		once.Do(func() { go func() { _ = srv.Shutdown(context.Background()) }() })
	}
	// The lab ends us with a request (labsite.StopPath), or by closing our
	// input when it is done with us, whatever it does with the process itself.
	site.OnStop(stop)
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		stop()
	}()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-interrupt
		stop()
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		stop()
		site.Close()
		fail("serve: %v", err)
	}
	site.Close()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "labserver: "+format+"\n", args...)
	os.Exit(1)
}
