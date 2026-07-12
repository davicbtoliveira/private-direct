package app

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var errPresenceSnapshotWrite = errors.New("write presence snapshot")

const webSocketWriteTimeout = 5 * time.Second

type wsJSONWriter interface {
	WriteJSON(any) error
	SetWriteDeadline(time.Time) error
	Close() error
}

type wsWriteRequest struct {
	message any
	result  chan bool
}

type presenceHub struct {
	mu              sync.RWMutex
	clients         map[int64]*wsClient
	pending         map[int64]*wsClient
	activationLocks sync.Map
}

type wsClient struct {
	user           authUser
	conn           wsJSONWriter
	messageMu      sync.Mutex
	closeOnce      sync.Once
	failureOnce    sync.Once
	active         atomic.Bool
	onWriteFailure func(*wsClient)

	outMu       sync.Mutex
	outCond     *sync.Cond
	outQueue    []wsWriteRequest
	outStarted  bool
	outRunning  bool
	outClosed   bool
	online      bool
	signalReady bool
	readyPeers  map[int64]struct{}
}

func newPresenceHub() *presenceHub {
	return &presenceHub{
		clients: map[int64]*wsClient{},
		pending: map[int64]*wsClient{},
	}
}

// activate publishes an initializing client before loading contacts. Events
// arriving during bootstrap queue behind the snapshot instead of being lost.
func (h *presenceHub) activate(client *wsClient, loadContacts func() ([]authUser, error)) (bool, error) {
	activationLock := h.activationLock(client.user.ID)
	activationLock.Lock()
	defer activationLock.Unlock()

	h.mu.Lock()
	previous := h.clients[client.user.ID]
	wasOnline := previous != nil && previous.online
	h.pending[client.user.ID] = client
	h.mu.Unlock()

	contacts, err := loadContacts()
	if err != nil {
		h.rollbackActivation(client)
		return false, err
	}

	h.mu.Lock()
	onlineUsers := h.prepareSnapshotLocked(client, contacts)
	h.mu.Unlock()

	if !client.initialize(presenceSnapshotEvent{
		Type:        "presence_snapshot",
		OnlineUsers: onlineUsers,
	}) {
		h.rollbackActivation(client)
		return false, errPresenceSnapshotWrite
	}
	h.mu.Lock()
	client.signalReady = true
	h.mu.Unlock()

	if previous != nil {
		previous.active.Store(false)
		previous.messageMu.Lock()
	}

	h.mu.Lock()
	if h.pending[client.user.ID] != client || h.clients[client.user.ID] != previous {
		h.mu.Unlock()
		if previous != nil {
			previous.active.Store(true)
			previous.messageMu.Unlock()
		}
		h.rollbackActivation(client)
		return false, errors.New("realtime activation changed")
	}
	delete(h.pending, client.user.ID)
	client.active.Store(true)
	client.online = true
	h.clients[client.user.ID] = client
	becameOnline := !wasOnline
	if becameOnline {
		peerIDs := make(map[int64]struct{}, len(contacts)+len(client.readyPeers))
		for _, contact := range contacts {
			peerIDs[contact.ID] = struct{}{}
		}
		for peerID := range client.readyPeers {
			peerIDs[peerID] = struct{}{}
		}
		client.readyPeers = nil
		for peerID := range peerIDs {
			h.enqueueForUserLocked(peerID, presenceEvent{
				Type:   "presence",
				User:   client.user,
				Online: true,
			})
		}
	}
	h.mu.Unlock()
	client.startOutput()

	if previous != nil {
		previous.writeJSON(sessionReplacedEvent{Type: "session_replaced"})
		previous.close()
		previous.messageMu.Unlock()
	}
	return becameOnline, nil
}

func (h *presenceHub) rollbackActivation(client *wsClient) {
	h.mu.Lock()
	if h.pending[client.user.ID] == client {
		delete(h.pending, client.user.ID)
	}
	h.mu.Unlock()
	client.close()
}

func (h *presenceHub) activationLock(userID int64) *sync.Mutex {
	lock, _ := h.activationLocks.LoadOrStore(userID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (h *presenceHub) remove(client *wsClient) bool {
	activationLock := h.activationLock(client.user.ID)
	activationLock.Lock()
	defer activationLock.Unlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.user.ID] != client {
		return false
	}
	client.active.Store(false)
	wasOnline := client.online
	delete(h.clients, client.user.ID)
	return wasOnline
}

func (h *presenceHub) sendToUser(userID int64, message any) bool {
	h.mu.RLock()
	client := h.clients[userID]
	queuedForReplacement := false
	if client == nil || !client.active.Load() {
		if candidate := h.pending[userID]; candidate != nil && candidate.signalReady {
			client = candidate
			queuedForReplacement = true
		} else {
			client = nil
		}
	}
	var result <-chan bool
	if client != nil {
		result = client.enqueue(message)
	}
	h.mu.RUnlock()
	if result == nil {
		return false
	}
	if queuedForReplacement {
		return true
	}
	return <-result
}

func (h *presenceHub) notifyUser(userID int64, message any) {
	h.mu.Lock()
	h.enqueueForUserLocked(userID, message)
	h.mu.Unlock()
}

func (h *presenceHub) sendCurrentPresence(userID int64, user authUser) bool {
	h.mu.Lock()
	onlineClient := h.clients[user.ID]
	online := onlineClient != nil && onlineClient.online
	initializingClient := h.pending[user.ID]
	if !online && initializingClient != nil {
		if initializingClient.readyPeers == nil {
			initializingClient.readyPeers = make(map[int64]struct{})
		}
		initializingClient.readyPeers[userID] = struct{}{}
	}
	message := presenceEvent{Type: "presence", User: user, Online: online}
	deliveries := h.enqueueForUserLocked(userID, message)
	h.mu.Unlock()
	return deliveries > 0
}

func (h *presenceHub) sendPresenceToContacts(user authUser, online bool, contactIDs []int64) {
	h.mu.Lock()
	client := h.clients[user.ID]
	actualOnline := client != nil && client.online
	var results []<-chan bool
	if actualOnline == online {
		for _, contactID := range contactIDs {
			message := presenceEvent{Type: "presence", User: user, Online: online}
			if target := h.clients[contactID]; target != nil {
				results = append(results, target.enqueue(message))
			}
			if pending := h.pending[contactID]; pending != nil {
				results = append(results, pending.enqueue(message))
			}
		}
	}
	h.mu.Unlock()
	for _, result := range results {
		<-result
	}
}

func (h *presenceHub) enqueueForUserLocked(userID int64, message any) int {
	deliveries := 0
	if client := h.clients[userID]; client != nil {
		client.enqueue(message)
		deliveries++
	}
	if pending := h.pending[userID]; pending != nil {
		pending.enqueue(message)
		deliveries++
	}
	return deliveries
}

func (h *presenceHub) prepareSnapshotLocked(client *wsClient, contacts []authUser) []authUser {
	peers := make(map[int64]authUser, len(contacts))
	for _, contact := range contacts {
		peers[contact.ID] = contact
	}
	for _, eventUser := range client.discardQueuedPresence() {
		peers[eventUser.ID] = eventUser
	}

	onlineUsers := make([]authUser, 0, len(peers))
	for _, peer := range peers {
		if target := h.clients[peer.ID]; target != nil && target.online {
			onlineUsers = append(onlineUsers, peer)
		}
	}
	sort.Slice(onlineUsers, func(i, j int) bool {
		return onlineUsers[i].Username < onlineUsers[j].Username
	})
	return onlineUsers
}

func (h *presenceHub) isOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client := h.clients[userID]
	return client != nil && client.online
}

func (c *wsClient) initialize(snapshot presenceSnapshotEvent) bool {
	c.outMu.Lock()
	if c.outClosed || c.outStarted {
		c.outMu.Unlock()
		return false
	}
	c.ensureOutputCondLocked()
	c.outStarted = true
	c.outMu.Unlock()

	err := c.conn.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout))
	if err == nil {
		err = c.conn.WriteJSON(snapshot)
	}
	if err != nil {
		c.close()
		return false
	}
	return true
}

func (c *wsClient) discardQueuedPresence() []authUser {
	c.outMu.Lock()
	defer c.outMu.Unlock()

	users := make([]authUser, 0)
	retained := c.outQueue[:0]
	for _, request := range c.outQueue {
		if event, ok := request.message.(presenceEvent); ok {
			users = append(users, event.User)
			request.result <- true
			continue
		}
		retained = append(retained, request)
	}
	c.outQueue = retained
	return users
}

func (c *wsClient) startOutput() {
	c.outMu.Lock()
	if c.outClosed || c.outRunning {
		c.outMu.Unlock()
		return
	}
	c.ensureOutputCondLocked()
	c.outRunning = true
	go c.writeLoop()
	c.outCond.Signal()
	c.outMu.Unlock()
}

func (c *wsClient) enqueue(message any) <-chan bool {
	result := make(chan bool, 1)
	c.outMu.Lock()
	if c.outClosed {
		result <- false
		c.outMu.Unlock()
		return result
	}
	c.ensureOutputCondLocked()
	c.outQueue = append(c.outQueue, wsWriteRequest{message: message, result: result})
	if c.outRunning {
		c.outCond.Signal()
	}
	c.outMu.Unlock()
	return result
}

func (c *wsClient) ensureOutputCondLocked() {
	if c.outCond == nil {
		c.outCond = sync.NewCond(&c.outMu)
	}
}

func (c *wsClient) writeLoop() {
	for {
		c.outMu.Lock()
		for len(c.outQueue) == 0 && !c.outClosed {
			c.outCond.Wait()
		}
		if c.outClosed {
			c.outMu.Unlock()
			return
		}
		request := c.outQueue[0]
		c.outQueue = c.outQueue[1:]
		c.outMu.Unlock()

		err := c.conn.SetWriteDeadline(time.Now().Add(webSocketWriteTimeout))
		if err == nil {
			err = c.conn.WriteJSON(request.message)
		}
		if err != nil {
			c.fail()
			request.result <- false
			return
		}
		request.result <- true
	}
}

func (c *wsClient) writeJSON(message any) bool {
	return <-c.enqueue(message)
}

func (c *wsClient) processMessage(process func()) bool {
	c.messageMu.Lock()
	defer c.messageMu.Unlock()
	if !c.active.Load() {
		return false
	}
	process()
	return true
}

func (c *wsClient) fail() {
	c.failureOnce.Do(func() {
		c.active.Store(false)
		c.close()
		if c.onWriteFailure != nil {
			go c.onWriteFailure(c)
		}
	})
}

func (c *wsClient) close() {
	c.closeOnce.Do(func() {
		c.outMu.Lock()
		c.outClosed = true
		pending := c.outQueue
		c.outQueue = nil
		if c.outCond != nil {
			c.outCond.Broadcast()
		}
		c.outMu.Unlock()
		for _, request := range pending {
			request.result <- false
		}
		_ = c.conn.Close()
	})
}
