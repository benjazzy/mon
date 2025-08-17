package shared

type ClientCommand string

const(
	Ping ClientCommand		= "ping"
	Reboot ClientCommand	= "reboot"
	Shutdown ClientCommand	= "shutdown"
)
