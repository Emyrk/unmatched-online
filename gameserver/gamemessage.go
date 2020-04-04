package gameserver

import (
	"encoding/json"
)

const (
	MsgTypePlayerState = "playerstate"
	MsgTypeGameState   = "gamestate"
)

type GameMessage struct {
	MessageType string          `json:"msgtype"`
	Content     json.RawMessage `json:"content"`
	Error       string          `json:"error,omitempty"`
}
