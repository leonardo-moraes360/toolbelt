package busrun

import (
	"github.com/leonardo-moraes360/toolbelt/internal/bus"
)

const PacketVersion = "bus-wake/0.1"

// Caps the headers listed in a packet.
const maxPacketMessages = 20

// Packet is the notice handed to the adapter. It never carries payloads.
type Packet struct {
	V        string       `json:"v"`
	Channel  string       `json:"channel"`
	Agent    string       `json:"agent"`
	Reason   string       `json:"reason"`
	Attempt  int          `json:"attempt"`
	Count    int          `json:"count"`
	FirstSeq int64        `json:"first_seq"`
	LastSeq  int64        `json:"last_seq"`
	Messages []PacketItem `json:"messages"`
	Recv     string       `json:"recv"`
}

type PacketItem struct {
	ID     string   `json:"id"`
	Seq    int64    `json:"seq"`
	From   string   `json:"from"`
	Type   bus.Type `json:"type"`
	Thread string   `json:"thread,omitempty"`
}

func newPacket(m bus.Member, reason string, msgs []bus.Message) *Packet {
	p := &Packet{
		V: PacketVersion, Channel: m.Channel, Agent: m.ID(), Reason: reason,
		Count: len(msgs), Recv: "tb bus recv --channel " + m.Channel + " --as " + m.Name,
		Messages: []PacketItem{},
	}
	if len(msgs) == 0 {
		return p
	}
	p.FirstSeq, p.LastSeq = msgs[0].Seq, msgs[len(msgs)-1].Seq
	for i := range msgs {
		if i == maxPacketMessages {
			break
		}
		p.Messages = append(p.Messages, PacketItem{
			ID: msgs[i].ID, Seq: msgs[i].Seq, From: msgs[i].From, Type: msgs[i].Type, Thread: msgs[i].Thread,
		})
	}
	return p
}
