package relay

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// ACK is acknowledgement message type.
	ACK = "ack"
	// OFFER is offer message type.
	OFFER = "offer"
	// ANSWER is answer message type.
	ANSWER = "answer"
	// CANDIDATE is candidate message type.
	CANDIDATE = "candidate"
	// INFO is informational message type.
	INFO = "info"
	// ONLINEPEERS lists peers that are online.
	ONLINEPEERS = "onlinePeers"
)

// IncomingACKMessage represents a received informational message.
type IncomingInfoMessage struct {
	Type    string              `json:"type"`
	Nonce   int                 `json:"nonce"`
	Payload IncomingInfoPayload `json:"payload"`
}

// IncomingInfoPayload represents a received informational payload.
type IncomingInfoPayload struct {
	ToID int         `json:"toAccountId"`
	Info interface{} `json:"info"`
}

// OutgoingInfoPayload represents an outgoing candidate payload.
type OutgoingInfoPayload struct {
	FromID int         `json:"fromAccountId"`
	Info   interface{} `json:"info"`
}

type outgoingInfoMessage struct {
	Type    string              `json:"type"`
	Nonce   int                 `json:"nonce"`
	Payload OutgoingInfoPayload `json:"payload"`
}

// IncomingACKMessage represents a received acknowledgement message.
type IncomingACKMessage struct {
	Type  string `json:"type"`
	Nonce int    `json:"nonce"`
}

type outgoingACKMessage struct {
	Type  string `json:"type"`
	Nonce int    `json:"nonce"`
}

type outgoingOnlinePeersMessage struct {
	Type    string                     `json:"type"`
	Nonce   int                        `json:"nonce"`
	Payload OutgoingOnlinePeersPayload `json:"payload"`
}

// OutgoingOfferPayload represents an payload with a list of online peers.
type OutgoingOnlinePeersPayload struct {
	OnlinePeers []int `json:"onlinePeers"`
}

// IncomingOfferPayload represents a received offer payload.
type IncomingOfferPayload struct {
	ToID  int         `json:"toAccountId"`
	Offer interface{} `json:"offer"`
}

// IncomingOfferMessage represents a received offer message.
type IncomingOfferMessage struct {
	Type    string               `json:"type"`
	Nonce   int                  `json:"nonce"`
	Payload IncomingOfferPayload `json:"payload"`
}

// OutgoingOfferPayload represents an outgoing offer payload.
type OutgoingOfferPayload struct {
	FromID int         `json:"fromAccountId"`
	Offer  interface{} `json:"offer"`
}

type outgoingOfferMessage struct {
	Type    string               `json:"type"`
	Nonce   int                  `json:"nonce"`
	Payload OutgoingOfferPayload `json:"payload"`
}

// IncomingAnswerPaylaod represents a received answer payload.
type IncomingAnswerPaylaod struct {
	ToID   int         `json:"toAccountId"`
	Answer interface{} `json:"answer"`
}

// IncomingAnswerMessage represents a received answer message.
type IncomingAnswerMessage struct {
	Type    string                `json:"type"`
	Nonce   int                   `json:"nonce"`
	Payload IncomingAnswerPaylaod `json:"payload"`
}

// OutgoingAnswerPayload represents an outgoing answer payload.
type OutgoingAnswerPayload struct {
	FromID int         `json:"fromAccountId"`
	Answer interface{} `json:"answer"`
}

type outgoingAnswerMessage struct {
	Type    string                `json:"type"`
	Nonce   int                   `json:"nonce"`
	Payload OutgoingAnswerPayload `json:"payload"`
}

// IncomingCandidatePayload represents a received candidate payload.
type IncomingCandidatePayload struct {
	ToID      int         `json:"toAccountId"`
	Candidate interface{} `json:"candidate"`
}

// IncomingCandidateMessage represents an outgoing candidate message.
type IncomingCandidateMessage struct {
	Type    string                   `json:"type"`
	Nonce   int                      `json:"nonce"`
	Payload IncomingCandidatePayload `json:"payload"`
}

// OutgoingCandidatePayload represents an outgoing candidate payload.
type OutgoingCandidatePayload struct {
	FromID    int         `json:"fromAccountId"`
	Candidate interface{} `json:"candidate"`
}

type outgoingCandidateMessage struct {
	Type    string                   `json:"type"`
	Nonce   int                      `json:"nonce"`
	Payload OutgoingCandidatePayload `json:"payload"`
}

// Conn represents a relay connection.
type Conn struct {
	// Lock for the underlying websocket connection reader.
	rLock sync.Mutex
	// Lock for the underlying websocket connection writer.
	wLock sync.Mutex
	// Lock for the Conn struct itself.
	rwMutex           sync.RWMutex
	conn              *websocket.Conn
	id                int
	lastOutgoingNonce int
	unackedNonces     map[int]*time.Timer
	// offersFor lists for whom the connection has offers.
	offersFor map[int]bool
	// expectingAnswersFrom lists from whom the connection is expecting answers,
	// after delivering its offers.
	expectingAnswersFrom map[int]bool
	// establishedWith lists with whom the connection has been established after
	// the offer-answer exchange.
	establishedWith   map[int]bool
	mostRecentMessage time.Time
}

func (c *Conn) IsOnline() bool {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()
	return time.Since(c.mostRecentMessage) < 20*time.Second
}

// Close closes the connection.
func (c *Conn) Close() {
	c.rwMutex.Lock()
	for _, timer := range c.unackedNonces {
		// Stop all acknowledgement timers.
		timer.Stop()
	}
	c.rwMutex.Unlock()

	c.rLock.Lock()
	defer c.rLock.Unlock()
	c.wLock.Lock()
	defer c.wLock.Unlock()

	c.conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(1*time.Second))
	c.conn.Close()
}

// Read reads from the connection.
func (c *Conn) Read() ([]byte, error) {
	c.rLock.Lock()
	defer c.rLock.Unlock()

	messageType, p, err := c.conn.ReadMessage()
	if err != nil {
		return []byte{}, err
	} else if messageType != websocket.TextMessage {
		return []byte{}, nil
	}

	// If we read a message of non-zero length, update the most recent
	// timestamp.
	if len(p) != 0 {
		c.mostRecentMessage = time.Now()
	}

	return p, nil
}

func (c *Conn) GetPeers() []int {
	uniquePeers := make(map[int]bool)
	for id := range c.offersFor {
		uniquePeers[id] = true
	}
	for id := range c.expectingAnswersFrom {
		uniquePeers[id] = true
	}
	for id := range c.establishedWith {
		uniquePeers[id] = true
	}

	peers := make([]int, 0, len(uniquePeers))
	for id := range uniquePeers {
		peers = append(peers, id)
	}
	return peers
}

// NewConn creates a connection.
func NewConn(id int, wsConn *websocket.Conn) *Conn {
	c := Conn{
		conn:                 wsConn,
		id:                   id,
		lastOutgoingNonce:    0,
		unackedNonces:        make(map[int]*time.Timer),
		offersFor:            make(map[int]bool),
		expectingAnswersFrom: make(map[int]bool),
		establishedWith:      make(map[int]bool),
	}

	wsConn.SetPongHandler(func(appData string) error {
		c.rwMutex.Lock()
		defer c.rwMutex.Unlock()
		c.mostRecentMessage = time.Now()
		return nil
	})

	return &c
}

// MarkAcked marks a message as ACKed.
func (c *Conn) MarkAcked(nonce int) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()
	if timer, ok := c.unackedNonces[nonce]; ok {
		timer.Stop()
		delete(c.unackedNonces, nonce)
	}
}

// StoreOffer stores the fact that a connection has an offer pending for a peer.
func (c *Conn) StoreOffer(peerID int, offer interface{}) {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()
	c.offersFor[peerID] = true
}

// IsEstablishedWith returns whether the connection has exchanged information
// with a particular peer.
func (c *Conn) IsEstablishedWith(peerID int) bool {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()
	return c.establishedWith[peerID]
}

// IsExpectingOfferFrom returns whether the connection is expecting an offer
// from a particular peer.
func (c *Conn) IsExpectingOfferFrom(peerID int) bool {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()
	return c.offersFor[peerID]
}

// IsExpectingAnswerFrom returns whether the connection is expecting an answer
// from a particular peer.
func (c *Conn) IsExpectingAnswerFrom(peerID int) bool {
	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()
	return c.expectingAnswersFrom[peerID]
}

// RelayAnswer relays an answer to a peer connection.
func (c *Conn) RelayAnswer(peer *Conn, answer interface{}) {
	msg := outgoingAnswerMessage{
		Type:  ANSWER,
		Nonce: peer.lastOutgoingNonce + 1,
		Payload: OutgoingAnswerPayload{
			FromID: c.id,
			Answer: answer,
		},
	}

	json, err := json.Marshal(msg)
	if err != nil {
		log.Print(err)
		return
	}

	peer.wLock.Lock()
	if err := peer.conn.WriteMessage(websocket.TextMessage, json); err != nil {
		log.Print(err)
		peer.wLock.Unlock()
		return
	}
	peer.wLock.Unlock()

	c.rwMutex.Lock()
	c.establishedWith[peer.id] = true
	c.rwMutex.Unlock()

	peer.rwMutex.Lock()
	defer peer.rwMutex.Unlock()
	peer.establishedWith[c.id] = true
	delete(peer.expectingAnswersFrom, c.id)
	peer.lastOutgoingNonce++
	peer.unackedNonces[peer.lastOutgoingNonce] = time.AfterFunc(1*time.Minute, func() {
		log.Printf("Never received ACK to message %v from account %v", msg, peer.id)
	})
}

// RelayInfo relays an airbitrary message to a peer connection.
func (c *Conn) RelayInfo(peer *Conn, info interface{}) {
	msg := outgoingInfoMessage{
		Type:  INFO,
		Nonce: peer.lastOutgoingNonce + 1,
		Payload: OutgoingInfoPayload{
			FromID: c.id,
			Info:   info,
		},
	}

	json, err := json.Marshal(msg)
	if err != nil {
		log.Print(err)
		return
	}

	peer.wLock.Lock()
	if err := peer.conn.WriteMessage(websocket.TextMessage, json); err != nil {
		log.Print(err)
		peer.wLock.Unlock()
		return
	}
	peer.wLock.Unlock()

	peer.rwMutex.Lock()
	defer peer.rwMutex.Unlock()
	peer.lastOutgoingNonce++
	peer.unackedNonces[peer.lastOutgoingNonce] = time.AfterFunc(1*time.Minute, func() {
		log.Printf("Never received ACK to message %v from account %v", msg, peer.id)
	})
}

// RelayCandidate relays a candidate to a peer connection.
func (c *Conn) RelayCandidate(peer *Conn, candidate interface{}) {
	msg := outgoingCandidateMessage{
		Type:  CANDIDATE,
		Nonce: peer.lastOutgoingNonce + 1,
		Payload: OutgoingCandidatePayload{
			FromID:    c.id,
			Candidate: candidate,
		},
	}

	json, err := json.Marshal(msg)
	if err != nil {
		log.Print(err)
		return
	}

	peer.wLock.Lock()
	if err := peer.conn.WriteMessage(websocket.TextMessage, json); err != nil {
		log.Print(err)
		peer.wLock.Unlock()
		return
	}
	peer.wLock.Unlock()

	peer.rwMutex.Lock()
	defer peer.rwMutex.Unlock()
	peer.lastOutgoingNonce++
	peer.unackedNonces[peer.lastOutgoingNonce] = time.AfterFunc(1*time.Minute, func() {
		log.Printf("Never received ACK to message %v from account %v", msg, peer.id)
	})
}

// RelayOffer relays an offer to a peer connection.
func (c *Conn) RelayOffer(peer *Conn, offer interface{}) {
	msg := outgoingOfferMessage{
		Type:  OFFER,
		Nonce: peer.lastOutgoingNonce + 1,
		Payload: OutgoingOfferPayload{
			FromID: c.id,
			Offer:  offer,
		},
	}

	json, err := json.Marshal(msg)
	if err != nil {
		log.Print(err)
		return
	}

	peer.wLock.Lock()
	if err := peer.conn.WriteMessage(websocket.TextMessage, json); err != nil {
		log.Print(err)
		peer.wLock.Unlock()
		return
	}
	peer.wLock.Unlock()

	peer.rwMutex.Lock()
	peer.lastOutgoingNonce++
	peer.unackedNonces[peer.lastOutgoingNonce] = time.AfterFunc(1*time.Minute, func() {
		log.Printf("Never received ACK to message %v from account %v", msg, peer.id)
	})
	peer.rwMutex.Unlock()

	c.rwMutex.Lock()
	c.expectingAnswersFrom[peer.id] = true
	c.rwMutex.Unlock()
}

// Ping sends a ping down the underlying connection.
func (c *Conn) Ping() {
	c.wLock.Lock()
	if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
		log.Print(err)
		return
	}
	c.wLock.Unlock()
}

// SendAck sends an acknowledgement message.
func (c *Conn) SendAck(nonce int) {
	json, err := json.Marshal(outgoingACKMessage{
		Type:  ACK,
		Nonce: nonce,
	})

	if err != nil {
		log.Print(err)
		return
	}

	c.wLock.Lock()
	defer c.wLock.Unlock()
	if err := c.conn.WriteMessage(websocket.TextMessage, json); err != nil {
		log.Print(err)
		return
	}
}

// SendOnlinePeers send a list of peers that are online.
func (c *Conn) SendOnlinePeers(onlinePeers []int) {
	// Perform this check so that Go encodes the empty slice as [], not null.
	if onlinePeers == nil {
		onlinePeers = make([]int, 0)
	}

	msg := outgoingOnlinePeersMessage{
		Type:  ONLINEPEERS,
		Nonce: c.lastOutgoingNonce + 1,
		Payload: OutgoingOnlinePeersPayload{
			OnlinePeers: onlinePeers,
		},
	}

	json, err := json.Marshal(msg)
	if err != nil {
		log.Print(err)
		return
	}

	c.wLock.Lock()
	if err := c.conn.WriteMessage(websocket.TextMessage, json); err != nil {
		log.Print(err)
		c.wLock.Unlock()
		return
	}
	c.wLock.Unlock()

	c.rwMutex.Lock()
	c.lastOutgoingNonce++
	c.unackedNonces[c.lastOutgoingNonce] = time.AfterFunc(1*time.Minute, func() {
		log.Printf("Never received ACK to message %v from account %v", msg, c.id)
	})
	c.rwMutex.Unlock()
}
