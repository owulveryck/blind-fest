package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"regexp"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"nhooyr.io/websocket"
)

// hub enables broadcasting to a set of subscribers.
type hub struct {
	// subscriberMessageBuffer controls the max number
	// of messages that can be queued for a subscriber
	// before it is kicked.
	//
	// Defaults to 16.
	subscriberMessageBuffer int

	// publishLimiter controls the rate limit applied to the publish endpoint.
	//
	// Defaults to one publish every 100ms with a burst of 8.
	publishLimiter *rate.Limiter

	// logf controls where logs are sent.
	// Defaults to log.Printf.
	logf func(f string, v ...interface{})

	// serveMux routes the various endpoints to the appropriate handler.
	serveMux http.ServeMux

	subscribersMu sync.Mutex
	subscribers   map[*subscriber]struct{}
}

// subscriber represents a subscriber.
// Messages are sent on the messageChan channel and if the client
// cannot keep up with the messages, closeSlow is called.
type subscriber struct {
	messageChan chan []byte
	closeSlow   func()
}

func (s *subscriber) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%p", s))
}

var validPath = regexp.MustCompile(`^/([0-9]+)`)

type gameServer struct {
	sessions *referencesDB
}

func newGamesServer(ctx context.Context) *gameServer {
	return &gameServer{
		sessions: newReferencesDB(ctx),
	}
}

func (g *gameServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	elements := validPath.FindStringSubmatch(r.URL.Path)
	if len(elements) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	key := elements[len(elements)-1]
	// find the key
	cs := g.sessions.getHub(key)
	if cs == nil {
		cs = &hub{
			subscriberMessageBuffer: 16,
			logf:                    log.Printf,
			subscribers:             make(map[*subscriber]struct{}),
			publishLimiter:          rate.NewLimiter(rate.Every(time.Millisecond*100), 8),
			serveMux:                *http.NewServeMux(),
		}
		cs.serveMux.HandleFunc(path.Join("/", key, "/subscribe"), cs.subscribeHandler)
		cs.serveMux.HandleFunc(path.Join("/", key, "/publish"), cs.publishHandler)
		g.sessions.addHub(key, cs)
	}
	cs.ServeHTTP(w, r)
}

func (cs *hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cs.serveMux.ServeHTTP(w, r)
}

// subscribeHandler accepts the WebSocket connection and then subscribes
// it to all future messages.
func (cs *hub) subscribeHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: config.OriginPatterns,
	})
	if err != nil {
		cs.logf("%v", err)
		return
	}
	defer func() {
		c.Close(websocket.StatusInternalError, "")
		b, _ := json.Marshal(&Info{
			c:       c,
			message: "left",
		})
		cs.publish(r.Context(), b)
	}()
	b, _ := json.Marshal(&Info{
		c:       c,
		message: "joined",
	})
	cs.publish(r.Context(), b)

	err = cs.subscribe(r.Context(), c)
	if errors.Is(err, context.Canceled) {
		return
	}
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}
	if err != nil {
		cs.logf("%v", err)
		return
	}
}

// publishHandler reads the request body with a limit of 8192 bytes and then publishes
// the received message.
func (cs *hub) publishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	body := http.MaxBytesReader(w, r.Body, 8192)
	msg, err := ioutil.ReadAll(body)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
		return
	}

	cs.publish(r.Context(), msg)

	w.WriteHeader(http.StatusAccepted)
}

// subscribe the given WebSocket to all broadcast messages.
// It creates a subscriber with a buffered messageChan chan to give some room to slower
// connections and then registers the subscriber. It then listens for all messages
// and writes them to the WebSocket. If the context is cancelled or
// an error occurs, it returns and deletes the subscription.
//
// It uses CloseRead to keep reading from the connection to process control
// messages and cancel the context if the connection drops.
func (cs *hub) subscribe(ctx context.Context, c *websocket.Conn) error {
	ctx = c.CloseRead(ctx)

	s := &subscriber{
		messageChan: make(chan []byte, cs.subscriberMessageBuffer),
		closeSlow: func() {
			err := c.Close(websocket.StatusPolicyViolation, "connection too slow to keep up with messages")
			if err != nil {
				log.Println(err)
			}
		},
	}
	cs.addSubscriber(s)
	defer cs.deleteSubscriber(s)

	for {
		select {
		case msg := <-s.messageChan:
			err := writeTimeout(ctx, time.Second*5, c, msg)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// publish the msg to all subscribers.
// It never blocks and so messages to slow subscribers
// are dropped.
func (cs *hub) publish(ctx context.Context, msg []byte) {
	cs.subscribersMu.Lock()
	defer cs.subscribersMu.Unlock()

	err := cs.publishLimiter.Wait(ctx)
	if err != nil {
		log.Println(err)
	}

	for s := range cs.subscribers {
		select {
		case s.messageChan <- msg:
		default:
			go s.closeSlow()
		}
	}
}

// addSubscriber registers a subscriber.
func (cs *hub) addSubscriber(s *subscriber) {
	cs.subscribersMu.Lock()
	cs.subscribers[s] = struct{}{}
	cs.subscribersMu.Unlock()
}

// deleteSubscriber deletes the given subscriber.
func (cs *hub) deleteSubscriber(s *subscriber) {
	cs.subscribersMu.Lock()
	delete(cs.subscribers, s)
	cs.subscribersMu.Unlock()
}

func writeTimeout(ctx context.Context, timeout time.Duration, c *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return c.Write(ctx, websocket.MessageText, msg)
}
