package session

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"

	"github.com/laenzlinger/go-midi-rtp/sip"
)

type connections []*MidiNetworkConnection

// MidiNetworkSession can offer or accept streams.
type MidiNetworkSession struct {
	LocalNaame  string
	BonjourName string
	Port        int
	connections connections
}

// Start is starting a new session
func Start(bonjourName string, port int) (s MidiNetworkSession) {
	s = MidiNetworkSession{
		BonjourName: bonjourName,
		Port:        port,
	}

	go messageLoop(port, s)

	go messageLoop(port+1, s)

	return
}

func messageLoop(port int, s MidiNetworkSession) {
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
		}
		log.Printf("received message: %v", msg)

		found, conn := s.connections.findConnection(msg.Name)
		if !found {
			conn = create(msg, &s)
			s.connections = append(s.connections, conn)
		}
		conn.HandleControl(msg, pc, addr)
	}
}

func (c connections) findConnection(remoteName string) (found bool, conn *MidiNetworkConnection) {
	// FIXME synchronisation issue
	found = false
	for _, conn = range c {
		// FIXME improve the connection identifaction
		if conn.Host.BonjourName == remoteName {
			return
		}
	}
	return
}
