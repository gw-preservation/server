package main

import (
	"fmt"
	"time"
)

type instance struct {
	mapId                  int
	alive                  bool
	gracefulShutdownSignal chan bool
	forceShutdownSignal    chan bool
}

func (i *instance) mainLoop() {
	for {
		select {
		case <-i.gracefulShutdownSignal:
			fmt.Printf("graceful shutdown for instance %d\n", i.mapId)
			if i.mapId != 165 {
				i.alive = false
			}
			return
		case <-i.forceShutdownSignal:
			fmt.Printf("force shutdown for instance %d\n", i.mapId)
			i.alive = false
			return
		default:
			time.Sleep(time.Second * 1)
			fmt.Printf("mainLoop from instance %v\n", i)
		}
	}
}

type mgr struct {
	instances map[int]*instance
}

func (m *mgr) addInstance(i *instance) {
	if m.hasInstance(i.mapId) {
		return
	}
	m.instances[i.mapId] = i
	i.alive = true
	go i.mainLoop()
}

func (m *mgr) hasInstance(mapId int) bool {
	inst, ok := m.instances[mapId]
	return ok && inst.alive
}

func (m *mgr) shutdown() {
	for mapId, inst := range m.instances {
		fmt.Printf("Sending graceful shutdown to instance for map %d\n", mapId)
		inst.gracefulShutdownSignal <- true
	}
}

func (m *mgr) forceShutdown(mapId int) {
	fmt.Printf("Sending force shutdown to instance for map %d\n", mapId)
	inst, ok := m.instances[mapId]
	if !ok {
		return
	}
	inst.forceShutdownSignal <- true
}

func main() {
	fmt.Printf("Test for creating isolated instances with their own main loop\n")
	manager := mgr{
		instances: map[int]*instance{},
	}

	for i := range 200 {
		if !manager.hasInstance(i) {
			// instance for map 1
			inst := instance{
				mapId:                  i,
				gracefulShutdownSignal: make(chan bool, 1),
				forceShutdownSignal:    make(chan bool, 1),
			}
			manager.addInstance(&inst)
		}
	}

	time.Sleep(time.Second * 4)
	manager.shutdown()
	time.Sleep(time.Second * 4)

	for mapId, inst := range manager.instances {
		if inst.alive {
			fmt.Printf("instance for mapId %d still alive! Sending force shutdown!\n", mapId)
			manager.forceShutdown(mapId)
		}
	}
}
