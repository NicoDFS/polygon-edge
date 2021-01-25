package blockchain

import (
	"sync"

	"github.com/0xPolygon/minimal/types"
)

type Subscription struct {
	updateCh chan struct{}
	closeCh  chan struct{}
	elem     *eventElem
}

func (s *Subscription) GetEvent() *Event {
	for {
		if s.elem.next != nil {
			s.elem = s.elem.next
			evnt := s.elem.event
			return evnt
		}

		// wait for an update
		select {
		case <-s.updateCh:
			continue
		case <-s.closeCh:
			return nil
		}
	}
}

func (s *Subscription) Close() {
	close(s.closeCh)
}

/*
TODO:
Different types for the event:
- Sealer
- Batch Sync
- Sync
- Manual?
Subscribe for specific event types. For example, the sealer might want
to get notified for head chain Sync events and the sync protocol (whichever it is)
wants to know about Sealer block events that it has to send to the network.
*/

type Event struct {
	// Old chain removed if there was a reorg
	OldChain []*types.Header

	// New part of the chain (or a fork)
	NewChain []*types.Header
}

func (e *Event) AddNewHeader(h *types.Header) {
	hh := h.Copy()
	if e.NewChain == nil {
		e.NewChain = []*types.Header{}
	}
	e.NewChain = append(e.NewChain, hh)
}

func (e *Event) AddOldHeader(h *types.Header) {
	hh := h.Copy()
	if e.OldChain == nil {
		e.OldChain = []*types.Header{}
	}
	e.OldChain = append(e.OldChain, hh)
}

func (b *Blockchain) SubscribeEvents() *Subscription {
	return b.stream.subscribe()
}

type eventElem struct {
	event *Event
	next  *eventElem
}

type eventStream struct {
	lock sync.Mutex
	head *eventElem

	// channel to notify updates
	updateCh []chan struct{}
}

func (e *eventStream) subscribe() *Subscription {
	head, updateCh := e.Head()
	s := &Subscription{
		elem:     head,
		updateCh: updateCh,
	}
	return s
}

func (e *eventStream) Head() (*eventElem, chan struct{}) {
	e.lock.Lock()
	head := e.head

	ch := make(chan struct{})
	if e.updateCh == nil {
		e.updateCh = []chan struct{}{}
	}
	e.updateCh = append(e.updateCh, ch)

	e.lock.Unlock()
	return head, ch
}

func (e *eventStream) push(event *Event) {
	e.lock.Lock()
	newHead := &eventElem{
		event: event,
	}
	if e.head != nil {
		e.head.next = newHead
	}
	e.head = newHead

	// notify the subscriptors
	for _, update := range e.updateCh {
		select {
		case update <- struct{}{}:
		default:
		}
	}
	e.lock.Unlock()
}