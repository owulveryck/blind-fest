package dispatcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
)

func TestDispatcher(t *testing.T) {
	ts := httptest.NewServer(NewGamesHandler(context.TODO()))
	res, err := http.Post(ts.URL, "text", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("exected %v, got %v", http.StatusNotFound, res.StatusCode)
	}
	res, err = http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("exected %v, got %v", http.StatusNotFound, res.StatusCode)
	}
	dialer := &websocket.Dialer{}
	testURL, _ := url.Parse(ts.URL)
	testURL.Scheme = "ws"
	testURL.Path = "/1234"
	conn1, res, err := dialer.Dial(testURL.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("exected %v, got %v", http.StatusSwitchingProtocols, res.StatusCode)
	}
	_ = conn1
	conn2, res, err := dialer.Dial(testURL.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("exected %v, got %v", http.StatusSwitchingProtocols, res.StatusCode)
	}
	conn3, res, err := dialer.Dial(testURL.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("exected %v, got %v", http.StatusSwitchingProtocols, res.StatusCode)
	}
	if err := conn1.WriteMessage(websocket.BinaryMessage, []byte(`bla`)); err != nil {
		t.Fatal(err)
	}
	_, p, err := conn2.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(p) != "bla" {
		t.Errorf("bad message: %s", p)
	}
	_, p, err = conn3.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if string(p) != "bla" {
		t.Errorf("bad message: %s", p)
	}
}
