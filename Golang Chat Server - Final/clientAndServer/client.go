package ClientAndServer

import (
	"github.com/gorilla/websocket"
)

type Client struct { // stores client's websocket connection and username
	conn *websocket.Conn
	name string
}

type Message struct {
	Message string `json:"message"` // contents of the message
	Name    string `json:"name"`    // sender's name
	Target  string `json:"target"`  // recipient's name. if the message is intended for everyone, Target will be ALL
	Type    string `json:"type"`    // the type of message. Some messages are sent by the server for various purposes
	Channel string `json:"channel"` // the channel the message is being sent to. The sender must have joined the channel to be able to send messages to it
}

func NewClient(conn *websocket.Conn) Client { //creates a new client for the server
	cc := Client{conn, ""}
	return cc
}
