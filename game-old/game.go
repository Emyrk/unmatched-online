package game_old

import (
	"sync"

	"github.com/pschlump/socketio"
)

type GameState struct {
	sync.RWMutex

	GameID      string
	Players     []*Player
	Spectators  []*Player
	Commitments []Commitment
	NameSpace   socketio.Namespace

	broadcast *socketio.Server
}

type Player struct {
	PlayerID int64
	Name     string
	RawInput string

	Socket socketio.Socket
}

func NewPlayer(so socketio.Socket) *Player {
	p := new(Player)
	p.Socket = so
	return p
}

func NewGame(namespace socketio.Namespace, broadcast *socketio.Server) *GameState {
	g := new(GameState)
	g.GameID = namespace.Name()
	g.NameSpace = namespace
	g.broadcast = broadcast

	// Add all game listeners
	g.NameSpace.On("testchat", func(msg string) {
		g.broadcast.BroadcastTo(g.GameID, msg)
	})

	return g
}

func (g *GameState) PlayerJoin(p *Player) error {
	g.Lock()
	defer g.Unlock()
	// TODO: Ensure player is unique
	g.Players = append(g.Players, p)

	p.Socket.Join("chat")
	return p.Socket.Join(g.GameID)
}

type Commitment struct {
	PlayerID int64
	MainCard string
	Boost    string
}
