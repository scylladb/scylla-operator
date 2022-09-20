package transport

import (
	"fmt"
	"log"
	"time"
)

var Now = time.Now

const UnknownShard = uint16(1<<16 - 1)

type ConnEvent struct {
	Addr  string
	Shard uint16
}

func (ev ConnEvent) String() string {
	return fmt.Sprintf("[addr=%s shard=%d]", ev.Addr, ev.Shard)
}

type span struct {
	Start time.Time
	End   time.Time
}

func startSpan() span {
	return span{
		Start: Now(),
	}
}

func (s *span) stop() {
	s.End = Now()
}

func (s *span) Duration() time.Duration {
	return s.End.Sub(s.Start)
}

type ConnectEvent struct {
	ConnEvent
	span

	// Err is the connection error (if any).
	Err error
}

type ConnObserver interface {
	OnConnect(ev ConnectEvent)
	OnPickReplacedWithLessBusyConn(ev ConnEvent)
}

type LoggingConnObserver struct{}

var _ ConnObserver = LoggingConnObserver{}

func (o LoggingConnObserver) OnConnect(ev ConnectEvent) {
	if ev.Err != nil {
		log.Printf("%s failed to open connection after %s: %s", ev, ev.Duration(), ev.Err)
	} else {
		log.Printf("%s connected in %s", ev, ev.Duration())
	}
}

func (o LoggingConnObserver) OnPickReplacedWithLessBusyConn(ev ConnEvent) {
	log.Printf("%s pick replaced with less busy conn", ev)
}
