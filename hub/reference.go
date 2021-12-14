package main

import (
	"context"
)

// References is a
type referencesDB struct {
	inputC          chan<- msg
	requestElementC chan<- requestElement
}

type msg struct {
	key   string
	value *hub
}

type requestElement struct {
	key    string
	replyC chan *hub
}

func newReferencesDB(ctx context.Context) *referencesDB {
	inputC := make(chan msg)
	requestElementC := make(chan requestElement)
	go func() {
		references := make(map[string]*hub)
		for {
			select {
			case <-ctx.Done():
				return
			case ref := <-inputC:
				references[ref.key] = ref.value
			case req := <-requestElementC:
				// if we don't expect a reply, we want to delete the key
				if req.replyC == nil {
					delete(references, req.key)
					continue
				}
				req.replyC <- references[req.key]
			}
		}

	}()
	return &referencesDB{
		inputC:          inputC,
		requestElementC: requestElementC,
	}
}

func (r *referencesDB) addHub(key string, d *hub) {
	r.inputC <- msg{key, d}
}

func (r *referencesDB) getHub(key string) *hub {
	replyC := make(chan *hub)
	r.requestElementC <- requestElement{
		key:    key,
		replyC: replyC,
	}
	d := <-replyC
	return d
}

func (r *referencesDB) removeHub(key string) {
	r.requestElementC <- requestElement{
		key:    key,
		replyC: nil,
	}
}
