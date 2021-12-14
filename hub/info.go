package main

import (
	"fmt"

	"nhooyr.io/websocket"
)

type Info struct {
	c       *websocket.Conn
	message string
}

func (c *Info) MarshalJSON() ([]byte, error) {
	payload := fmt.Sprintf(`{"%p": "%v"}`, c.c, c.message)
	return []byte(payload), nil
}
