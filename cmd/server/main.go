package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"zapping-task/internal/api"
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

	pool, err := stream.LoadPool(segmentsDir + "/segment.m3u8")
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	conn, err := db.Connect(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	liveState := stream.NewLiveState(pool)
	liveState.StartTicker(10 * time.Second)
	fmt.Println("segments loaded:", len(pool.Segments), "- target duration:", pool.TargetDuration)
	fmt.Println("connected to postgres")

	liveStream := api.NewStream(pool, liveState, segmentsDir)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, liveStream, webDir)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
