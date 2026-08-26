package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"zapping-task/internal/api"
	"zapping-task/internal/auth"
	"zapping-task/internal/db"
	"zapping-task/internal/stream"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	segmentsDir := env("SEGMENTS_DIR", "hls test/hls test")
	databaseURL := env("DATABASE_URL", "postgres://zapping:zapping@localhost:5432/zapping")
	const webDir = "web"

	/*Load the pool of segments*/
	pool, err := stream.LoadPool(segmentsDir + "/segment.m3u8")
	if err != nil {
		log.Fatal(err)
	}

	/*Bounds the startup connection to Postgres. It expires after 5s,
	so it must not be reused for queries later on*/
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	store := db.NewStore(conn)
	authenticator := auth.New(store, env("COOKIE_SECURE", "true") == "true")

	/*LiveState simulates a live stream over the static pool*/
	liveState := stream.NewLiveState(pool)
	/*Advances the window one segment every 10s, on its own goroutine
	so it never blocks the server*/
	liveState.StartTicker(10 * time.Second)
	fmt.Println("segments loaded:", len(pool.Segments), "- target duration:", pool.TargetDuration)
	fmt.Println("connected to postgres")

	/*Initialize Stream handlers*/
	liveStream := api.NewStream(pool, liveState, segmentsDir)

	/*Custom Mux to register custom routes, DefaultServeMux is global and any package can register routes*/
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, liveStream, authenticator, webDir)

	/*ReadHeaderTimeout is what stops a slowloris attack. WriteTimeout is a
	hard deadline on the whole response, so SegmentHandler extends its own*/
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
