package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"practice-run/chat"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/sync/errgroup"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var chatService = chat.NewChat()

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	http.HandleFunc("/ws", serveWs(ctx, chatService))

	server := &http.Server{Addr: ":8080"}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	cancel()
	log.Println("Shutting down server...")

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")

}

func serveWs(ctx context.Context, chatService *chat.Chat) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}

		participant := chat.NewParticipant(conn, chatService)
		defer participant.Close()

		errGroup, ctx := errgroup.WithContext(ctx)
		participant.HandleMessages(ctx, errGroup)
		participant.HandleEvents(ctx, errGroup)
		if err := errGroup.Wait(); err != nil {
			log.Println("participant error:", err)
		}
	}
}
