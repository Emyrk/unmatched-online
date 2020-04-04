package main

import (
	"fmt"

	"github.com/Emyrk/unmatched-online/gameserver"
)

func main() {
	gs := gameserver.NewGameServer()
	fmt.Println(gs.Serve())
}
