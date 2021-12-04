package dispatcher

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
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

// a hub actually broadcasts all the events it receives on a websocket to the others
type hub struct {
	cancelFunc          context.CancelFunc
	receiveC            chan []byte
	regiterParticipantC chan *client
	participants        []*client
}

func (h *hub) startBroadcast(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case participant := <-h.regiterParticipantC:
			h.participants = append(h.participants, participant)
		case message := <-h.receiveC:
			for i := range h.participants {
				select {
				case h.participants[i].sendC <- message:
				case <-ctx.Done():
					return
				}
			}
		}
	}

}

func newHub(ctx context.Context) *hub {
	_, cancel := context.WithCancel(ctx)
	h := &hub{
		cancelFunc:          cancel,
		receiveC:            make(chan []byte, 256), // buffered channel
		regiterParticipantC: make(chan *client),
		participants:        make([]*client, 0),
	}
	go h.startBroadcast(ctx)
	return h
}

func (h *hub) addParticipant(w http.ResponseWriter, r *http.Request, responseHeader http.Header) error {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		return err
	}
	client := &client{
		hub:   h,
		sendC: make(chan []byte, 256),
		conn:  conn,
	}
	select {
	case <-r.Context().Done():
		return r.Context().Err()
	case h.regiterParticipantC <- client:
	}
	go client.listen(r.Context())
	go client.send(r.Context())
	return nil

}

type client struct {
	conn  *websocket.Conn
	sendC chan []byte
	hub   *hub
}

func (c *client) send(_ context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.sendC:
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

			// Add queued messages to the current websocket message.
			n := len(c.sendC)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.sendC)
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

func (c *client) listen(_ context.Context) {
	defer func() {
		c.conn.Close()
		for i := range c.hub.participants {
			if c.hub.participants[i] == c {
				c.hub.participants = append(c.hub.participants[:i], c.hub.participants[i+1:]...)
				// if we were the last connection, close the hub
				if len(c.hub.participants) == 0 {
					c.hub.cancelFunc()
				}
			}
		}
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
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.hub.receiveC <- message
	}
}
