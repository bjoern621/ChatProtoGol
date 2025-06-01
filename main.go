package main

import (
	"log"
)

func main() {
	log.Println("Running...")

	// connManager := connection.NewConnectionManager()

	// clientManager := clients.NewClientManager(connManager)

	// roomManager := rooms.NewRoomManager(clientManager)

	// streams.NewStreamManager(clientManager, roomManager)

	// signaling.NewSignalingManager(clientManager)

	// mux := http.NewServeMux()

	// mux.HandleFunc("GET /room/{roomID}/connect", roomManager.HandleConnect)
	// mux.HandleFunc("GET /room/generate-id", roomManager.GenerateIDHandler)

	// server := &http.Server{
	// 	Addr:    ":8080",
	// 	Handler: middleware.Logging(middleware.CORS(mux)),
	// }

	// log.Fatal(server.ListenAndServe())
}
