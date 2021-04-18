package client

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return 0 == 0
	},
	WriteBufferSize: 1024,
	ReadBufferSize: 1024,
}


type Client struct {
	conn *websocket.Conn
	//Rooms map[*Room]bool
	ws *WsServer
	message chan []byte
}


func onNewClient(conn *websocket.Conn, ws *WsServer) *Client{
	return &Client{
		conn: conn,
		//Rooms: make(map[*Room]bool),
		ws: ws,
		message: make(chan []byte),
	}
}

//write and read message form client
func (c *Client) readPump() {
	defer func() {
		c.ws.unRegister <- c
		//
		for room := range c.ws.Rooms {
			room.UnRegister <- c
		}
		log.Println("from read message client room :",len(c.ws.Rooms))
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		//decode json message
		c.onJsonMessageCheck(message)
	}
}
//
// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.message:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.message)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.message)
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

//room action
//join room
func (c *Client) onClientJointRoom(message *Message){
	//find room
	room := c.ws.onFindRoomName(message.RoomName)
	if room == nil{
		//not found room give new create room
		room = c.ws.onCreateNewRoom(message.RoomName)
	}
	//register room
	c.ws.Rooms[room] = 0 == 0
	room.Register <- c

	log.Println("ws room :",len(c.ws.Rooms))
	//log.Println("ws room :",len(c.Rooms))
	log.Println("ws client:",len(c.ws.clients))
}
//leave room
func (c *Client) onClientLeaveRoom(message *Message){
	//find room
	room := c.ws.onFindRoomName(message.RoomName)
	//check room in list wsServer
	if _,ok := c.ws.Rooms[room]; ok{
		//if found remove it
		delete(c.ws.Rooms , room)
	}
	//unRegister room
	room.UnRegister <- c
}

//decode json message
//and check event from client
func (c *Client)onJsonMessageCheck(jsonData []byte)  {
	var data Message
	//json data from message
	if err := json.Unmarshal(jsonData , &data); err != nil{
		log.Println("decode data error :",err)
		return
	}
	//
	switch data.MessageType {
	case TypeRoom:
		log.Println("Type Room :")
		c.onClientJointRoom(&data)
		break
	case TypeLeaveRoom:
		log.Println("Type client leave from room")
		c.onClientLeaveRoom(&data)
		//client leave from room
		break
	case TypeMessage:
		log.Println("Type Message")
		//jsonData = bytes.TrimSpace(bytes.Replace(jsonData, newline, space, -1))
		if room := c.ws.onFindRoomName(data.RoomName); room != nil{
			room.OnMessage <- &data
		}else {
			log.Println("not room")
		}
		//c.ws.sendMessage <- &data
		break
	case TypeCallOffer:
		log.Println("Type webRTC offer")
		//webRTC offer
		break
	case TypeCallAnswer:
		log.Println("Type webRTC answer")
		//webRTC answer
		break
	case TypeJoinChannel:
		log.Println("Type webRTC create channel video call")
		//webRTC create channel video call
		break
	case TypeLeaveChannel:
		log.Println("Type client end video call")
		//client  end video call
		break
	default:
		break
	}
}


func OnWsServer(w http.ResponseWriter , r *http.Request , ws *WsServer){
	//
	conn , err := upgrader.Upgrade(w,r,nil)
	if err != nil{
		log.Fatal(err)
	}
	//new client
	client := onNewClient(conn,ws)
	//register
	client.ws.register <- client

	
	go client.writePump()
	go client.readPump()
}
