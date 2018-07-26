package rmutex

import (
	"sync"

	"github.com/v2pro/plz/gls"
)

type Mutex struct {
	cnt    uint32
	owner  int64
	locker sync.Locker
}

func NewMutex(locker sync.Locker) Mutex {
	return Mutex{
		cnt:    0,
		owner:  0,
		locker: locker,
	}
}

func (m *Mutex) Lock() {
	id := gls.GoID()
	if m.cnt != 0 && m.owner == id {
		m.cnt++
	} else {
		m.locker.Lock()
		m.owner = id
		m.cnt = 1
	}
}

func (m *Mutex) Unlock() {
	id := gls.GoID()
	if m.cnt == 0 || m.owner != id {
		panic("You are try to unlock a rmutex not owned")
	}

	m.cnt--
	if m.cnt == 0 {
		m.locker.Unlock()
	}
}
