package cmd

import (
	"bjoernblessin.de/chatprotogol/routing"
	"bjoernblessin.de/chatprotogol/sequencing"
	"bjoernblessin.de/chatprotogol/sock"
)

var socket sock.Socket
var router *routing.Router
var outSequencing *sequencing.OutgoingPktNumHandler

// SetGlobalVars sets the global socket variable to the provided socket.
func SetGlobalVars(s sock.Socket, r *routing.Router, out *sequencing.OutgoingPktNumHandler) {
	socket = s
	router = r
	outSequencing = out
}
