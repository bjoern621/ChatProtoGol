package cmd

import (
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sock"
)

var socket sock.Socket
var router *routing.Router

// SetGlobalVars sets the global socket variable to the provided socket.
func SetGlobalVars(s sock.Socket, r *routing.Router) {
	socket = s
	router = r
}
