package gameserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
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

	gs.HTTPServer.Handler = handlers.CORS()(gs.Mux)
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
	homeTemplate.Execute(w, "ws://a7ed8baa.ngrok.io/ws/"+gid)

}

func (gs *GameServer) WSHandler(w http.ResponseWriter, r *http.Request) {
	log.Info("Websocket attempt started")
	vars := mux.Vars(r)
	gid := vars["gid"]
	room, ok := gs.Rooms[gid]
	if !ok {
		_, _ = fmt.Fprintf(w, "Unable to find the game room socket: %v\n", fmt.Errorf("gid: %s", gid))
		return
	}

	c, err := room.WS.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.WithError(err).Errorf("failed to upgrade websocket")
		return
	}

	values, _ := url.ParseQuery(r.URL.RawQuery)
	if len(values["name"]) == 0 {
		w.WriteHeader(http.StatusFailedDependency)
		_, _ = fmt.Fprintf(w, "required 'playername' header not found")
		return
	}
	name := strings.Join(values["name"], " ")

	err = room.PlayerJoin(c, name)
	if err != nil {
		_, _ = fmt.Fprintf(w, "player failed to join: %s", err.Error())
		return
	}
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

type PlayerConn struct {
	Name string
	*websocket.Conn
}

type Room struct {
	GameID    string
	WS        *websocket.Upgrader
	Clients   map[string]*PlayerConn
	GameState map[string]json.RawMessage

	mutex sync.Mutex

	ctx  context.Context
	stop context.CancelFunc
}

func NewRoom(gid string, ctx context.Context) *Room {
	r := new(Room)
	r.GameID = gid
	r.GameState = make(map[string]json.RawMessage)
	r.WS = &websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return true
	}}
	r.ctx, r.stop = context.WithCancel(ctx)
	r.Clients = make(map[string]*PlayerConn)

	return r
}

func (r *Room) Close() {
	r.stop()
	for _, c := range r.Clients {
		_ = c.Close()
	}
}

func (r *Room) PlayerJoin(c *websocket.Conn, name string) error {
	player := &PlayerConn{
		Conn: c,
		Name: name,
	}

	if name == "" {
		return fmt.Errorf("must provide a player name")
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()
	_, ok := r.Clients[name]
	if ok {
		return fmt.Errorf("player name taken")
	}
	r.Clients[name] = player
	r.GameState[name] = []byte("{}")

	go r.PlayerListener(player, r.ctx)
	r.broadcastAll(websocket.TextMessage, r.GetGameState())

	return nil
}

func (r *Room) PlayerListener(c *PlayerConn, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			r.PlayerExit(c)
			return // Player closed
		default:
		}

		mt, message, err := c.ReadMessage()
		if err != nil {
			log.WithError(err).Error("read failed: player exited")
			r.PlayerExit(c)
			break
		}
		r.HandleMessage(c, mt, message)
	}
}

func (r *Room) HandleMessage(c *PlayerConn, mt int, msg []byte) {
	gm := new(GameMessage)
	err := json.Unmarshal(msg, gm)
	if err != nil {
		log.WithError(err).Errorf("msg from client not able to decode: %s", msg)
		return
	}
	switch gm.MessageType {
	case MsgTypePlayerState:
		// Update player state
		r.mutex.Lock()
		r.GameState[c.Name] = gm.Content
		msg := r.GetGameState()
		r.broadcastAll(websocket.TextMessage, msg)
		r.mutex.Unlock()
		log.Info("Player State Received")
	default:
		log.Errorf("msg type '%s' is undefined", gm.MessageType)
	}
}

func (r *Room) GetGameState() []byte {
	data, err := json.Marshal(r.GameState)
	if err != nil {
		log.WithError(err).Errorf("failed to marshal game state")
	}
	msg, err := json.Marshal(GameMessage{
		MessageType: MsgTypeGameState,
		Content:     data,
	})
	if err != nil {
		log.WithError(err).Errorf("failed to marshal game state")
	}
	return msg
}

func (r *Room) BroadcastAll(mt int, msg []byte) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.broadcastAll(mt, msg)
}

func (r *Room) broadcastAll(mt int, msg []byte) {
	for _, c := range r.Clients {
		err := c.WriteMessage(mt, msg)
		if err != nil {
			log.WithError(err).Error("write failed")
		}
	}
}

func (r *Room) PlayerExit(c *PlayerConn) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	_, ok := r.Clients[c.Name]
	delete(r.Clients, c.Name)

	return ok
}
