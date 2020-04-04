package gameserver

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
)

type GameServer struct {
	HTTPServer *http.Server
	Mux        *mux.Router

	Rooms map[string]*Room
}

func NewGameServer() *GameServer {
	gs := new(GameServer)
	gs.HTTPServer = &http.Server{}
	gs.Rooms = make(map[string]*Room)

	return gs
}

func (gs *GameServer) Serve() error {
	gs.Mux = mux.NewRouter()
	gs.Mux.HandleFunc("/lobby/{gid}", gs.LobbyHandler)
	gs.Mux.HandleFunc("/ws/{gid}", gs.WSHandler)

	gs.HTTPServer.Handler = gs.Mux
	gs.HTTPServer.Addr = "localhost:1111"

	return gs.HTTPServer.ListenAndServe()
}

func (gs *GameServer) LobbyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	gid := vars["gid"]
	new, err := gs.CreateLobby(gid)
	if err != nil {
		log.WithError(err).Errorf("failed to make the game room")
		fmt.Fprintf(w, "Unable to make the game room: %v\n", err)
		return
	}
	if new {
		log.WithFields(log.Fields{"gid": gid}).Info("new game room created")
	}

	// fmt.Fprintf(w, "Game ID: %v\n", vars["gid"])
	homeTemplate.Execute(w, "ws://localhost:1111/ws/"+gid)

}

func (gs *GameServer) WSHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("Websocket attempt started")
	vars := mux.Vars(r)
	gid := vars["gid"]
	room, ok := gs.Rooms[gid]
	if !ok {
		fmt.Fprintf(w, "Unable to find the game room socket: %v\n", fmt.Errorf("gid: %s", gid))
		return
	}

	c, err := room.WS.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.WithError(err).Errorf("failed to upgrade websocket")
		return
	}

	room.PlayerJoin(c)
}

func (gs *GameServer) CreateLobby(gid string) (bool, error) {
	if gid == "" {
		return false, fmt.Errorf("game id cannot be blank")
	}

	_, ok := gs.Rooms[gid]
	if ok {
		return false, nil
	}

	gs.Rooms[gid] = NewRoom(gid, context.Background())

	return true, nil
}

type Room struct {
	GameID  string
	WS      *websocket.Upgrader
	Clients []*websocket.Conn

	mutex sync.Mutex

	ctx  context.Context
	stop context.CancelFunc
}

func NewRoom(gid string, ctx context.Context) *Room {
	r := new(Room)
	r.GameID = gid
	r.WS = &websocket.Upgrader{}
	r.ctx, r.stop = context.WithCancel(ctx)

	return r
}

func (r *Room) Close() {
	r.stop()
	for _, c := range r.Clients {
		_ = c.Close()
	}
}

func (r *Room) PlayerJoin(c *websocket.Conn) {
	go r.PlayerListener(c, r.ctx)

	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.Clients = append(r.Clients, c)
}

func (r *Room) PlayerListener(c *websocket.Conn, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return // Player closed
		default:
		}

		mt, message, err := c.ReadMessage()
		if err != nil {
			log.WithError(err).Error("read failed: player exited")
			r.PlayerExit(c)
			break
		}
		r.Broadcast(mt, message)
	}
}

func (r *Room) Broadcast(mt int, msg []byte) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, c := range r.Clients {
		err := c.WriteMessage(mt, msg)
		if err != nil {
			log.WithError(err).Error("write failed")
		}
	}
}

func (r *Room) PlayerExit(c *websocket.Conn) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for i := range r.Clients {
		if r.Clients[i] == c {
			new := make([]*websocket.Conn, len(r.Clients)-1)
			copy(new[:i], r.Clients[:i])
			copy(new[i:], r.Clients[i+1:])
			r.Clients = new
			return true
		}
	}
	return false
}
