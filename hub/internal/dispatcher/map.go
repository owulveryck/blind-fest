package dispatcher

import (
	"context"
)

// References is a
type referencesDB struct {
	inputC          chan<- msg
	requestElementC chan<- requestElement
}

type msg struct {
	key   int
	value *hub
}

type requestElement struct {
	key    int
	replyC chan *hub
}

func newReferencesDB(ctx context.Context) *referencesDB {
	inputC := make(chan msg)
	requestElementC := make(chan requestElement)
	go func() {
		references := make(map[int]*hub)
		for {
			select {
			case <-ctx.Done():
				return
			case ref := <-inputC:
				references[ref.key] = ref.value
			case req := <-requestElementC:
				// if we dont expect a reply, we want to delete the key
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

func (r *referencesDB) addHub(key int, d *hub) {
	r.inputC <- msg{key, d}
}

func (r *referencesDB) getHub(key int) *hub {
	replyC := make(chan *hub)
	r.requestElementC <- requestElement{
		key:    key,
		replyC: replyC,
	}
	d := <-replyC
	return d
}

func (r *referencesDB) removeHub(key int) {
	r.requestElementC <- requestElement{
		key:    key,
		replyC: nil,
	}
}
