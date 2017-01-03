/*
Use this you must hack your go:

1. Add the code to the file $GOROOT/src/runtime/extern.go:
	func GoID() int64 {
	    return getg().goid
	}

2. Add those to the file $GOROOT/api/go1.txt
	pkg runtime, func GoID() int64
*/
package rmutex

import (
	"runtime"
	"sync"
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
	id := runtime.GoID()
	if m.cnt != 0 && m.owner == id {
		m.cnt++
	} else {
		m.locker.Lock()
		m.owner = id
		m.cnt = 1
	}
}

func (m *Mutex) Unlock() {
	id := runtime.GoID()
	if m.cnt == 0 || m.owner != id {
		panic("You are try to unlock a rmutex not owned")
	}

	m.cnt--
	if m.cnt == 0 {
		m.locker.Unlock()
	}
}
