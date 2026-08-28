package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"zapping-task/internal/api"
	"zapping-task/internal/auth"
	"zapping-task/internal/chat"
	"zapping-task/internal/config"
	"zapping-task/internal/db"
	"zapping-task/internal/stream"
)

const (
	dbConnectTimeout  = 5 * time.Second
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	/*Load the pool of segments*/
	pool, err := stream.LoadPool(filepath.Join(cfg.SegmentsDir, config.PlaylistFilename))
	if err != nil {
		log.Fatal(err)
	}

	/*Bounds the startup connection to Postgres. It expires after dbConnectTimeout,
	so it must not be reused for queries later on*/
	ctx, cancel := context.WithTimeout(context.Background(), dbConnectTimeout)
	defer cancel()

	conn, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	}

	store := db.NewStore(conn)
	authenticator := auth.New(store, cfg.SecureCookies)

	/*LiveState simulates a live stream over the static pool*/
	liveState := stream.NewLiveState(pool)
	/*Advances the window one segment every TickInterval, on its own goroutine
	so it never blocks the server*/
	liveState.StartTicker(stream.TickInterval)
	fmt.Println("segments loaded:", len(pool.Segments), "- target duration:", pool.TargetDuration)
	fmt.Println("connected to postgres")

	/*Initialize Stream handlers*/
	liveStream := api.NewStream(pool, liveState, cfg.SegmentsDir)

	chatState := chat.NewChatState(cfg.ChatHistorySize)
	liveChat := api.NewChat(chatState)

	/*Custom Mux to register custom routes, DefaultServeMux is global and any package can register routes*/
	mux := http.NewServeMux()
	api.RegisterRoutes(mux, liveStream, liveChat, authenticator, cfg.WebDir)

	/*ReadHeaderTimeout is what stops a slowloris attack. WriteTimeout is a
	hard deadline on the whole response, so SegmentHandler extends its own*/
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	fmt.Println("listening on", cfg.ListenAddr)
	log.Fatal(server.ListenAndServe())
}
