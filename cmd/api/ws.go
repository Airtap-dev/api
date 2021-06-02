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
	rwMutex     sync.RWMutex
	connections map[int]*relay.Conn
}

func init() {
	pool.connections = make(map[int]*relay.Conn)
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

	pool.rwMutex.Lock()
	relayConn := relay.NewConn(acc.id, wsConn)
	pool.connections[acc.id] = relayConn
	pool.rwMutex.Unlock()

	// Need to send ping messages every 30 seconds down the connection so that
	// Heroku doesn't reap it. In our case we are going to send messages with a
	// list of online peers every 15 seconds instead of pings.
	ticker := time.NewTicker(15 * time.Second)
	stopPing := make(chan bool)
	// Start a goroutine for pings.
	go func() {
		for {
			select {
			case <-ticker.C:
				relayConn.SendOnlinePeers(buildOnlinePeers(relayConn))
			case <-stopPing:
				return
			}
		}
	}()

	// Deallocate connection resources upon return.
	defer func() {
		pool.rwMutex.Lock()
		defer pool.rwMutex.Unlock()
		// Remove the connection from the pool
		delete(pool.connections, acc.id)
		// Deallocate the pinger goroutine.
		ticker.Stop()
		stopPing <- true
		// Close the connection.
		relayConn.Close()
	}()

	for {
		p, err := relayConn.Read()
		if err != nil {
			return nil, err
		} else if len(p) == 0 {
			continue
		}

		// Assume the message is ACK. Try to parse as such.
		ackMessage := relay.IncomingACKMessage{}
		err = json.Unmarshal(p, &ackMessage)
		if err == nil && strings.ToLower(ackMessage.Type) == relay.ACK {
			relayConn.MarkAcked(ackMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			// Since we don't know the message type and are trying to parse it
			// sequentially, an error of type UnmarshalTypeError simply means we
			// should carry on. Any other error, however, is problematic.
			log.Print(err)
			continue
		}

		// Assume the message is OFFER. Try to parse as such.
		offerMessage := relay.IncomingOfferMessage{}
		err = json.Unmarshal(p, &offerMessage)
		if err == nil && strings.ToLower(ackMessage.Type) == relay.OFFER {
			handleOffer(relayConn, offerMessage.Payload.Offer, acc.id, offerMessage.Payload.ToID)
			relayConn.SendAck(offerMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			// Since we don't know the message type and are trying to parse it
			// sequentially, an error of type UnmarshalTypeError simply means we
			// should carry on. Any other error, however, is problematic.
			log.Print(err)
			continue
		}

		// Assume the message is ANSWER. Try to parse as such.
		answerMessage := relay.IncomingAnswerMessage{}
		err = json.Unmarshal(p, &answerMessage)
		if err == nil && strings.ToLower(answerMessage.Type) == relay.ANSWER {
			handleAnswer(relayConn, answerMessage.Payload.Answer, acc.id, answerMessage.Payload.ToID)
			relayConn.SendAck(answerMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			// Since we don't know the message type and are trying to parse it
			// sequentially, an error of type UnmarshalTypeError simply means we
			// should carry on. Any other error, however, is problematic.
			log.Print(err)
			continue
		}

		// Assume the message is INFO. Try to parse as such.
		infoMessage := relay.IncomingInfoMessage{}
		err = json.Unmarshal(p, &infoMessage)
		if err == nil && strings.ToLower(infoMessage.Type) == relay.INFO {
			handleInfo(relayConn, infoMessage.Payload.Info, acc.id, answerMessage.Payload.ToID)
			relayConn.SendAck(infoMessage.Nonce)
			continue
		} else if _, ok := err.(*json.UnmarshalTypeError); err != nil && !ok {
			// Since we don't know the message type and are trying to parse
			// it sequentially, an error of type UnmarshalTypeError simply
			// means we should carry on. Any other error, however, is
			// problematic.
			log.Print(err)
			continue
		}

		// Assume the message is CANDIDATE. Try to parse as such.
		candidateMessage := relay.IncomingCandidateMessage{}
		err = json.Unmarshal(p, &candidateMessage)
		if err == nil && strings.ToLower(candidateMessage.Type) == relay.CANDIDATE {
			handleCandidate(relayConn, candidateMessage.Payload.Candidate, acc.id, candidateMessage.Payload.ToID)
			relayConn.SendAck(candidateMessage.Nonce)
			continue
		} else {
			// At this point the message has to parse as CANDIDATE. The entire
			// else clause is indicative of a problem.
			log.Printf("Unknown message type: %v, %v", err, string(p))
			continue
		}
	}
}

func handleOffer(conn *relay.Conn, offer interface{}, selfID, peerID int) {
	conn.StoreOffer(peerID, offer)

	pool.rwMutex.RLock()
	if peer, ok := pool.connections[peerID]; ok {
		pool.rwMutex.RUnlock()
		if peer.IsExpectingOfferFrom(selfID) {
			conn.RelayOffer(peer, offer)
		}
	} else {
		pool.rwMutex.RUnlock()
	}
}

func handleAnswer(conn *relay.Conn, answer interface{}, selfID, peerID int) {
	pool.rwMutex.RLock()
	if peer, ok := pool.connections[peerID]; ok {
		pool.rwMutex.RUnlock()
		if peer.IsExpectingAnswerFrom(selfID) {
			conn.RelayAnswer(peer, answer)
		}
	} else {
		pool.rwMutex.RUnlock()
	}
}

func handleInfo(conn *relay.Conn, info interface{}, selfID, peerID int) {
	pool.rwMutex.RLock()
	if peer, ok := pool.connections[peerID]; ok {
		pool.rwMutex.RUnlock()
		if peer.IsEstablishedWith(selfID) {
			conn.RelayInfo(peer, info)
		}
	} else {
		pool.rwMutex.RUnlock()
	}
}

func handleCandidate(conn *relay.Conn, candidate interface{}, selfID, peerID int) {
	pool.rwMutex.RLock()
	if peer, ok := pool.connections[peerID]; ok {
		pool.rwMutex.RUnlock()
		conn.RelayCandidate(peer, candidate)
	} else {
		pool.rwMutex.RUnlock()
	}
}

// This function takes a connection, builds a set of all peers it might be in
// communication with, and then returns a subset of that set with only those
// peers that are online.
func buildOnlinePeers(conn *relay.Conn) (onlinePeers []int) {
	peers := conn.GetPeers()
	pool.rwMutex.RLock()
	defer pool.rwMutex.RUnlock()
	for _, peerID := range peers {
		if c, ok := pool.connections[peerID]; ok && c.IsOnline() {
			onlinePeers = append(onlinePeers, peerID)
		}
	}

	return onlinePeers
}
