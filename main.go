package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Garage48/internal/camera"
	"Garage48/internal/server"
)

func main() {
	configPath := flag.String("config", "config.json", "path to config.json")
	bind := flag.String("bind", ":8080", "HTTP bind address")
	maxFPS := flag.Int("fps", 15, "max processing FPS per camera")
	detectorURL := flag.String("detector", "http://127.0.0.1:9000", "object detector base URL (Python sidecar)")
	flag.Parse()

	cfg, err := server.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if len(cfg.Cameras) == 0 {
		log.Fatalf("no cameras in %s", *configPath)
	}

	// Create registry with factory that builds pure-Go MJPEG cameras using the detector HTTP endpoint.
	reg := camera.NewRegistry(func(id, url string) *camera.Camera {
		return camera.NewCamera(id, url, *detectorURL, *maxFPS)
	})
	for _, c := range cfg.Cameras {
		if err := reg.AddCamera(c.ID, c.URL); err != nil {
			log.Printf("camera %s failed to start: %v", c.ID, err)
		} else {
			log.Printf("camera %s started: %s", c.ID, c.URL)
		}
	}
	defer reg.Close()

	srv := server.New(*bind, cfg, reg)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("http server stopped: %v", err)
		}
	}()
	fmt.Printf("Server listening on %s (detector: %s)\n", *bind, *detectorURL)

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
