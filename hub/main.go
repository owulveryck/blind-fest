package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/kelseyhightower/envconfig"
)

func main() {
	log.SetFlags(0)

	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

// run initializes the chatServer and then
// starts a http.Server for the passed in address.
func run() error {
	h := flag.Bool("h", false, "print usage")
	flag.Parse()
	if *h {
		envconfig.Usage("", &config)
		os.Exit(0)
	}
	err := envconfig.Process("", &config)
	if err != nil {
		envconfig.Usage("", &config)
		log.Fatal(err.Error())
	}
	l, err := net.Listen("tcp", config.Host)
	if err != nil {
		return err
	}
	log.Printf("listening on http://%v", l.Addr())
	ctx := context.Background()

	gameServer := newGamesServer(ctx)
	s := &http.Server{
		Handler:      gameServer,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	errc := make(chan error, 1)
	go func() {
		errc <- s.Serve(l)
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errc:
		log.Printf("failed to serve: %v", err)
	case sig := <-sigs:
		log.Printf("terminating: %v", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	return s.Shutdown(ctx)
}
