package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"server/lib/relay"

	"github.com/gorilla/websocket"
)

var pool struct {
	// TODO: change to RW lock for efficiency.
	mu          sync.Mutex
	connections map[int]*relay.Conn
}

func ws(acc account, w http.ResponseWriter, r *http.Request) (response, error) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(err)
		return nil, errInternal
	}

	pool.mu.Lock()
	relayConn := relay.NewConn(acc.id, wsConn)
	pool.connections[acc.id] = relayConn
	pool.mu.Unlock()

	defer func() {
		pool.mu.Lock()
		if c, ok := pool.connections[acc.id]; ok {
			c.Close()
		}
		delete(pool.connections, acc.id)
		pool.mu.Unlock()
	}()

	// Need to send a ping message down the connection so that Heroku doesn't
	// reap it.
	ticker := time.NewTicker(30 * time.Second)
	done := make(chan bool)
	defer func() {
		ticker.Stop()
		done <- true
	}()

	go func() {
		for {
			select {
			case <-ticker.C:
				relayConn.Ping()
			case <-done:
				return
			}
		}
	}()

	for {
		p, n := relayConn.Read()
		if n == -1 {
			break
		} else if n == 0 {
			continue
		}

		ackMessage := relay.IncomingACKMessage{}
		err := json.Unmarshal(p, &ackMessage)
		if err == nil && strings.ToLower(ackMessage.Type) == relay.ACK {
			relayConn.MarkAcked(ackMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			log.Print(err)
			continue
		}

		offerMessage := relay.IncomingOfferMessage{}
		err = json.Unmarshal(p, &offerMessage)
		if err == nil && strings.ToLower(ackMessage.Type) == relay.OFFER {
			handleOffer(relayConn, offerMessage.Payload.Offer, acc.id, offerMessage.Payload.ToID)
			relayConn.SendAck(offerMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			log.Print(err)
			continue
		}

		answerMessage := relay.IncomingAnswerMessage{}
		err = json.Unmarshal(p, &answerMessage)
		if err == nil && strings.ToLower(answerMessage.Type) == relay.ANSWER {
			handleAnswer(relayConn, answerMessage.Payload.Answer, acc.id, answerMessage.Payload.ToID)
			relayConn.SendAck(answerMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			log.Print(err)
			continue
		}

		candidateMessage := relay.IncomingCandidateMessage{}
		err = json.Unmarshal(p, &candidateMessage)
		if err == nil && strings.ToLower(candidateMessage.Type) == relay.CANDIDATE {
			handleCandidate(relayConn, candidateMessage.Payload.Candidate, acc.id, candidateMessage.Payload.ToID)
			relayConn.SendAck(candidateMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			log.Print(err)
			continue
		}
	}

	return nil, nil
}

func handleOffer(conn *relay.Conn, offer interface{}, selfID, peerID int) {
	conn.StoreOffer(peerID, offer)

	pool.mu.Lock()
	if peer, ok := pool.connections[peerID]; ok {
		pool.mu.Unlock()
		if peer.IsExpectingOfferFrom(selfID) {
			conn.RelayOffer(peer, offer)
		}
	} else {
		pool.mu.Unlock()
	}
}

func handleAnswer(conn *relay.Conn, answer interface{}, selfID, peerID int) {
	pool.mu.Lock()
	if peer, ok := pool.connections[peerID]; ok {
		pool.mu.Unlock()
		if peer.IsExpectingAnswerFrom(selfID) {
			conn.RelayAnswer(peer, answer)
		} else {
		}
	} else {
		pool.mu.Unlock()
	}
}

func handleCandidate(conn *relay.Conn, candidate interface{}, selfID, peerID int) {
	pool.mu.Lock()
	if peer, ok := pool.connections[peerID]; ok {
		pool.mu.Unlock()
		conn.RelayCandidate(peer, candidate)
	} else {
		pool.mu.Unlock()
	}
}
