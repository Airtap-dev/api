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
	mu          sync.Mutex
	connections map[int]*relay.Conn
}

func ws(acc account, w http.ResponseWriter, r *http.Request) {
	log.Printf("received new connection from %v", acc.id)
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print(err)
		return
	}

	pool.mu.Lock()
	relayConn := relay.NewConn(acc.id, wsConn)
	pool.connections[acc.id] = relayConn
	pool.mu.Unlock()

	defer func() {
		// TODO: close timeout goroutines
		log.Printf("closing connection with %v", acc.id)
		pool.mu.Lock()
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

		log.Printf("got a message from %v", acc.id)

		ackMessage := relay.IncomingACKMessage{}
		err := json.Unmarshal(p, &ackMessage)
		if err == nil && strings.ToLower(ackMessage.Type) == relay.ACK {
			log.Printf("ack")
			relayConn.MarkAcked(ackMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			log.Print(err)
			continue
		}

		offerMessage := relay.IncomingOfferMessage{}
		err = json.Unmarshal(p, &offerMessage)
		if err == nil && strings.ToLower(ackMessage.Type) == relay.OFFER {
			log.Printf("offer")
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
			log.Printf("answer")
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
			log.Printf("candidate")
			handleCandidate(relayConn, candidateMessage.Payload.Candidate, acc.id, candidateMessage.Payload.ToID)
			relayConn.SendAck(candidateMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			log.Print(err)
			continue
		}

		log.Printf("Unknown message type: %v", string(p))
	}
}

func handleOffer(conn *relay.Conn, offer interface{}, selfID, peerID int) {
	log.Printf("storing offer")
	conn.StoreOffer(peerID, offer)
	log.Printf("stored offer")

	pool.mu.Lock()
	if peer, ok := pool.connections[peerID]; ok {
		log.Printf("peer exists")
		pool.mu.Unlock()
		if peer.IsExpectingOfferFrom(selfID) {
			log.Printf("peer expecting")
			conn.RelayOffer(peer, offer)
		}
	} else {
		log.Printf("peer doesn't exist yet")
		pool.mu.Unlock()
	}
}

func handleAnswer(conn *relay.Conn, answer interface{}, selfID, peerID int) {
	pool.mu.Lock()
	if peer, ok := pool.connections[peerID]; ok {
		log.Println("peer exists")
		pool.mu.Unlock()
		if peer.IsExpectingAnswerFrom(selfID) {
			log.Println("peer expects answer")
			conn.RelayAnswer(peer, answer)
		} else {
			log.Println("peer not expecting answer")
		}
	} else {
		log.Println("peer doesn't exist")
		pool.mu.Unlock()
	}
}

func handleCandidate(conn *relay.Conn, candidate interface{}, selfID, peerID int) {
	pool.mu.Lock()
	if peer, ok := pool.connections[peerID]; ok {
		log.Println("peer exists")
		pool.mu.Unlock()
		conn.RelayCandidate(peer, candidate)
	} else {
		log.Println("peer doesn't exist")
		pool.mu.Unlock()
	}
}
