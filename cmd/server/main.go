package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"zapping-task/internal/api"
	"zapping-task/internal/stream"
)

func main() {

	const segmentsDir = "hls test/hls test"

	pool, err := stream.LoadPool(segmentsDir + "/segment.m3u8")

	if err != nil {
		log.Fatal(err)
	}
	liveState := stream.NewLiveState(pool)
	liveState.StartTicker(10 * time.Second)
	fmt.Println("segments loaded:", len(pool.Segments), "- target duration:", pool.TargetDuration)

	liveStream := api.NewStream(pool, liveState, segmentsDir)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux, liveStream)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
