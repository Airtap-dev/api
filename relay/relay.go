package relay

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	ACK       = "ack"
	OFFER     = "offer"
	ANSWER    = "answer"
	CANDIDATE = "candidate"
)

type IncomingACKMessage struct {
	Type  string `json:"type"`
	Nonce int    `json:"nonce"`
}

type outgoingACKMessage struct {
	Type  string `json:"type"`
	Nonce int    `json:"nonce"`
}

type IncomingOfferPayload struct {
	ToID  int         `json:"toAccountId"`
	Offer interface{} `json:"offer"`
}

type IncomingOfferMessage struct {
	Type    string               `json:"type"`
	Nonce   int                  `json:"nonce"`
	Payload IncomingOfferPayload `json:"payload,omitempty"`
}

type OutgoingOfferPayload struct {
	FromID int         `json:"fromAccountId"`
	Offer  interface{} `json:"offer"`
}

type outgoingOfferMessage struct {
	Type    string               `json:"type"`
	Nonce   int                  `json:"nonce"`
	Payload OutgoingOfferPayload `json:"payload,omitempty"`
}

type IncomingAnswerPaylaod struct {
	ToID   int         `json:"toAccountId"`
	Answer interface{} `json:"answer"`
}

type IncomingAnswerMessage struct {
	Type    string                `json:"type"`
	Nonce   int                   `json:"nonce"`
	Payload IncomingAnswerPaylaod `json:"payload,omitempty"`
}

type OutgoingAnswerPayload struct {
	FromID int         `json:"fromAccountId"`
	Answer interface{} `json:"answer"`
}

type outgoingAnswerMessage struct {
	Type    string                `json:"type"`
	Nonce   int                   `json:"nonce"`
	Payload OutgoingAnswerPayload `json:"payload,omitempty"`
}

type IncomingCandidatePayload struct {
	ToID      int         `json:"toAccountId"`
	Candidate interface{} `json:"candidate"`
}

type IncomingCandidateMessage struct {
	Type    string                   `json:"type"`
	Nonce   int                      `json:"nonce"`
	Payload IncomingCandidatePayload `json:"payload,omitempty"`
}

type OutgoingCandidatePayload struct {
	FromID    int         `json:"fromAccountId"`
	Candidate interface{} `json:"candidate"`
}

type outgoingCandidateMessage struct {
	Type    string                   `json:"type"`
	Nonce   int                      `json:"nonce"`
	Payload OutgoingCandidatePayload `json:"payload,omitempty"`
}

type Conn struct {
	rLock                sync.Mutex
	wLock                sync.Mutex
	mu                   sync.Mutex
	conn                 *websocket.Conn
	id                   int
	lastOutgoingNonce    int
	unackedNonces        map[int]*time.Timer
	offersFor            map[int]bool
	expectingAnswersFrom map[int]bool
}

func (c *Conn) Read() ([]byte, int) {
	c.rLock.Lock()
	defer c.rLock.Unlock()

	messageType, p, err := c.conn.ReadMessage()
	if err != nil {
		log.Print(err)
		return []byte{}, -1
	} else if messageType != websocket.TextMessage {
		log.Print("Received a non-text message")
		return []byte{}, 0
	}

	return p, len(p)
}

func NewConn(id int, wsConn *websocket.Conn) *Conn {
	return &Conn{
		conn:                 wsConn,
		id:                   id,
		lastOutgoingNonce:    0,
		unackedNonces:        make(map[int]*time.Timer),
		offersFor:            make(map[int]bool),
		expectingAnswersFrom: make(map[int]bool),
	}
}

func (c *Conn) MarkAcked(nonce int) {
	c.mu.Lock()
	if unacked, ok := c.unackedNonces[nonce]; ok {
		unacked.Stop()
		delete(c.unackedNonces, nonce)
	}
	c.mu.Unlock()
}

func (c *Conn) StoreOffer(peerID int, offer interface{}) {
	c.mu.Lock()
	c.offersFor[peerID] = true
	c.mu.Unlock()
}

func (c *Conn) IsExpectingOfferFrom(peerID int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ok := c.offersFor[peerID]; ok {
		return true
	}

	return false
}

func (c *Conn) IsExpectingAnswerFrom(peerID int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.expectingAnswersFrom[peerID]
}

func (c *Conn) RelayAnswer(peer *Conn, answer interface{}) {
	log.Println("relaying answer")
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

	peer.mu.Lock()
	delete(peer.expectingAnswersFrom, c.id)
	peer.lastOutgoingNonce++
	peer.unackedNonces[peer.lastOutgoingNonce] = time.AfterFunc(1*time.Minute, func() {
		log.Printf("Never received ACK to message %v from account %v", msg, peer.id)
	})
	peer.mu.Unlock()
}

func (c *Conn) RelayCandidate(peer *Conn, candidate interface{}) {
	log.Println("relaying candidate")
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

	peer.mu.Lock()
	peer.lastOutgoingNonce++
	peer.unackedNonces[peer.lastOutgoingNonce] = time.AfterFunc(1*time.Minute, func() {
		log.Printf("Never received ACK to message %v from account %v", msg, peer.id)
	})
	peer.mu.Unlock()
}

func (c *Conn) RelayOffer(peer *Conn, offer interface{}) {
	log.Printf("relaying offer")
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

	peer.mu.Lock()
	peer.lastOutgoingNonce++
	peer.unackedNonces[peer.lastOutgoingNonce] = time.AfterFunc(1*time.Minute, func() {
		log.Printf("Never received ACK to message %v from account %v", msg, peer.id)
	})
	peer.mu.Unlock()

	c.mu.Lock()
	c.expectingAnswersFrom[peer.id] = true
	c.mu.Unlock()
}

func (c *Conn) Ping() {
	c.wLock.Lock()
	if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
		log.Print(err)
		return
	}
	c.wLock.Unlock()
	log.Println("pinged")
}

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
	log.Println("sent ack")
}
