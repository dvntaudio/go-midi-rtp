package session

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/laenzlinger/go-midi-rtp/rtp"
	"github.com/laenzlinger/go-midi-rtp/sip"
)

// MidiNetworkSession can offer or accept streams.
type MidiNetworkSession struct {
	LocalNaame  string
	BonjourName string
	Port        uint16
	SSRC        uint32
	StartTime   time.Time
	connections sync.Map
}

// Start is starting a new session
func Start(bonjourName string, port uint16) (s *MidiNetworkSession) {
	ssrc := rand.Uint32()
	startTime := time.Now()
	session := MidiNetworkSession{
		BonjourName: bonjourName,
		SSRC:        ssrc,
		Port:        port,
		StartTime:   startTime,
	}

	go messageLoop(port, &session)

	go messageLoop(port+1, &session)

	return &session
}

// End is ending a session
func (s *MidiNetworkSession) End() {
	s.connections.Range(func(k, v interface{}) bool {
		v.(*MidiNetworkConnection).End()
		return true
	})
}

// SendMessage sends the MIDI payload immediately to all MidiNetworkConnections
func (s *MidiNetworkSession) SendMIDIMessage(payload []byte) {
	m := rtp.MIDIMessage{
		SequenceNumber: 1, // FIXME use random and increase for each message
		SSRC:           s.SSRC,
		Commands: rtp.MIDICommands{
			Timestamp: time.Now(),
			Commands:  []rtp.MIDICommand{{Payload: payload}},
		},
	}
	s.connections.Range(func(k, v interface{}) bool {
		v.(*MidiNetworkConnection).SendMIDIMessage(m)
		return true
	})
}

func messageLoop(port uint16, s *MidiNetworkSession) {
	pc, mcErr := net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	if mcErr != nil {
		panic(mcErr)
	}
	defer pc.Close()
	buffer := make([]byte, 1024)
	for {
		n, addr, err := pc.ReadFrom(buffer)
		fmt.Println(hex.Dump(buffer[:n]))
		if err != nil {
			fmt.Println(err)
			continue
		}

		msg, err := sip.Decode(buffer[:n])
		if err != nil {
			fmt.Println(err)
			continue
		}
		log.Printf("-> incoming message: %v", msg)

		s.getConnection(msg).HandleControl(msg, pc, addr)
	}
}

func (s *MidiNetworkSession) getConnection(msg sip.ControlMessage) *MidiNetworkConnection {
	// FIXME optimize to only create a session for IN message
	conn, found := s.connections.LoadOrStore(msg.SSRC, s.createConnection(msg))
	if !found {
		log.Printf("New connection requested from remote participant SSRC [%x]", msg.SSRC)
	}
	return conn.(*MidiNetworkConnection)
}

func (s *MidiNetworkSession) removeConnection(conn *MidiNetworkConnection) {
	log.Printf("Connection ended by remote participant SSRC [%x]", conn.RemoteSSRC)
	s.connections.Delete(conn.RemoteSSRC)
}

func (s *MidiNetworkSession) createConnection(msg sip.ControlMessage) *MidiNetworkConnection {
	host := MidiNetworkHost{BonjourName: msg.Name}
	conn := MidiNetworkConnection{
		Session:    s,
		Host:       host,
		RemoteSSRC: msg.SSRC,
		State:      initial,
	}
	return &conn
}
