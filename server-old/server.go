package server_old

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/Emyrk/unmatched-online/game-old"
	log "github.com/sirupsen/logrus"

	"github.com/pschlump/socketio"
)

func init() {
	socketio.DbLogMessage = false
	socketio.LogMessage = false
}

type GlobalServer struct {
	Rooms map[string]*game_old.GameState
	*socketio.Server
	ws websocket.Upgrader

	// Counters
	totalconnects int
	totaljoins    int
}

func (s *GlobalServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	origin := r.Header.Get("Origin")
	w.Header().Set("Access-Control-Allow-Origin", origin)
	s.Server.ServeHTTP(w, r)
}

func (s *GlobalServer) JoinRoom(roomid string, so socketio.Socket) (bool, error) {
	p := game_old.NewPlayer(so)
	if roomid == "" {
		return false, fmt.Errorf("Room id cannot be blank")
	}
	room, ok := s.Rooms[roomid]
	if !ok {
		s.Rooms[roomid] = game_old.NewGame(s.Of(roomid), s.Server)
		room = s.Rooms[roomid]
	}

	err := room.PlayerJoin(p)
	return !ok, err
}

func NewGameServer() (*GlobalServer, error) {
	server, err := socketio.NewServer(nil)
	if err != nil {
		return nil, err
	}

	gs := new(GlobalServer)
	gs.Rooms = make(map[string]*game_old.GameState)
	gs.Server = server

	gs.On("lobby", func(so socketio.Socket, msg string) {
		gid := msg // TODO: Validate
		new, err := gs.JoinRoom(gid, so)
		logf := log.WithFields(gs.LogFields()).WithFields(log.Fields{"gid": gid, "id": so.Id(), "rooms": so.Rooms()})
		if err != nil {
			logf.WithError(err).Error("failed to join room")
			return
		}

		if new {
			logf.Info("New room created")
		}
		gs.totaljoins++
		logf.Info("joined game")

		// TODO: Validation
		// TODO: Send over all game state for new player
	})

	gs.On("connection", func(so socketio.Socket) error {
		gs.totalconnects++
		fmt.Println("client connected:", so.Id())

		so.On("disconnect", func() {
			fmt.Println("disconnected:", so.Id())
		})

		return nil
	})

	// server.On("connection", func(so socketio.Socket) error {
	// 	so.On("")
	// 	fmt.Println("connected:", so.Id())
	// 	so.Join("chat")
	//
	// 	so.On("chat message", func(msg string) {
	// 		fmt.Println("Client Msg: ", msg)
	// 		fmt.Println(so.BroadcastTo("chat", "chat message", msg))
	// 		// server.BroadcastTo("chat message", msg)
	// 		// so.BroadcastTo("chat", "chat message", msg)
	// 	})
	//
	// 	so.On("disconnect", func() {
	// 		fmt.Println("disconnected:", so.Id())
	// 	})
	//
	// 	return nil
	// })

	// go server.Serve()

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("Hello"))
	})

	http.Handle("/socket.io/", gs)
	log.Info("Serving")
	log.Fatal(http.ListenAndServe(":8000", nil))
	return nil, nil
}

func (gs *GlobalServer) LogFields() log.Fields {
	return log.Fields{
		"rooms":         len(gs.Rooms),
		"totaljoins":    gs.totaljoins,
		"totalconnects": gs.totalconnects,
	}
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
}
