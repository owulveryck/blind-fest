package dispatcher

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"github.com/gorilla/websocket"
)

var (
	validPath = regexp.MustCompile(`^/([0-9])+$`)

	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// GamesHandler is an http handler the reads /{:id:}/ and route the messages the disaptchers accordingly
type GamesHandler struct {
	sessions *referencesDB
}

func NewGamesHandler(ctx context.Context) *GamesHandler {
	return &GamesHandler{
		sessions: newReferencesDB(ctx),
	}
}

func (d *GamesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// failfast, if it is not a get request
	if r.Method != http.MethodGet {
		http.Error(w, "unsupported", http.StatusMethodNotAllowed)
		return
	}
	elements := validPath.FindStringSubmatch(r.URL.Path)
	if len(elements) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	key, err := strconv.Atoi(elements[len(elements)-1])
	if err != nil {
		panic(err)
	}
	// find the key
	game := d.sessions.getHub(key)
	if game == nil {
		game = newHub(context.TODO())
		d.sessions.addHub(key, game)
		// TODO: remove this hub when not used anymore
	}
	err = game.addParticipant(w, r, nil)
	if err != nil {
		log.Printf("cannot add participant: %v", err)
		return
	}
}
