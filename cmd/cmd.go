package cmd

import (
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/skt"
)

var socket skt.Socket
var router *routing.Router

// SetGlobalVars sets the global socket variable to the provided socket.
func SetGlobalVars(s skt.Socket, r *routing.Router) {
	socket = s
	router = r
}
