package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/owulveryck/blind-fest/hub/internal/dispatcher"
)

var addr = flag.String("addr", ":8080", "http service address")

func main() {
	flag.Parse()
	ctx := context.Background()
	games := dispatcher.NewGamesHandler(ctx)
	log.Println("listening on ws://" + *addr + "/[0.9]+")
	err := http.ListenAndServe(*addr, games)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
