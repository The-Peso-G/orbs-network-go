package events

import (
	"fmt"
	"time"
	"sync"
)

type Events interface {
	Report(message string)
}

const FinishedConsensusRound = "finished_consensus_round"
const ConsensusError = "consensus_error"

type Latch interface {
	Events

	WaitFor(message string)
}

type latch struct {
	cond       *sync.Cond
	waitingFor string
}

func NewLatch() Latch {
	return &latch{}
}

func (l *latch) WaitFor(message string) {
	l.waitingFor = message
	mutex := &sync.Mutex{}
	mutex.Lock()
	l.cond = sync.NewCond(mutex)
	l.cond.Wait()
}

func (l *latch) Report(message string) () {
	if l.waitingFor == message && l.cond != nil {
		l.cond.Broadcast()
		l.cond = nil
		l.waitingFor = ""
	}
}

type BufferedLog interface {
	Events

	Flush()
}

type bufferedLog struct {
	loggedEvents []string
	name         string
}

func NewBufferedLog(name string) BufferedLog {
	e := bufferedLog{name: name}
	e.log("Start of log")
	return &e
}

func (e *bufferedLog) Flush() {
	for _, line := range e.loggedEvents {
		println(line)
	}
}

func (e *bufferedLog) Report(message string) () {
	e.log(message)
}

func (e *bufferedLog) log(message string) {
	e.loggedEvents = append(e.loggedEvents, fmt.Sprintf("[%s] [%s]: %s", e.name, time.Now().Format("15:04:05.99999999"), message))
}

type compositeEvents struct {
	children []Events
}

func NewCompositeEvents(children []Events) Events {
	return &compositeEvents{children: children}
}

func (e *compositeEvents) Report(message string) () {
	for _, child := range e.children {
		child.Report(message)
	}
}
