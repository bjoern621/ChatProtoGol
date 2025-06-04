package cmd

func HandleDisconnect(args []string) {
	if len(args) == 0 {
		println("Usage: disconnect <IPv4 address> <port> Example: disconnect")
		return
	}
	println("Verbindung getrennt")
}
