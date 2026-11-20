package busrun

import (
	"sort"

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
	// Types and Senders count every message of the batch (Messages stops at
	// maxPacketMessages), so a notification can be built from the packet.
	Types       map[bus.Type]int `json:"types"`
	Senders     []SenderCount    `json:"senders"`
	SendersMore int              `json:"senders_more,omitempty"`
	Recv        string           `json:"recv"`
}

type SenderCount struct {
	From  string `json:"from"`
	Count int    `json:"count"`
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
		Messages: []PacketItem{}, Types: map[bus.Type]int{}, Senders: []SenderCount{},
	}
	if len(msgs) == 0 {
		return p
	}
	p.Senders, p.SendersMore = countSenders(msgs, p.Types)
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

// countSenders fills types and returns the senders sorted by count, then
// name, capped at maxPacketMessages plus how many were left out.
func countSenders(msgs []bus.Message, types map[bus.Type]int) (out []SenderCount, more int) {
	by := map[string]int{}
	for i := range msgs {
		types[msgs[i].Type]++
		by[msgs[i].From]++
	}
	out = make([]SenderCount, 0, len(by))
	for from, n := range by {
		out = append(out, SenderCount{From: from, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].From < out[j].From
	})
	if len(out) > maxPacketMessages {
		more = len(out) - maxPacketMessages
		out = out[:maxPacketMessages]
	}
	return out, more
}
