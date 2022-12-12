package logic

import (
	"fmt"
	"sync"

	"github.com/hedon954/go-lock-step-server/logic/room"
)

// RoomManager is used to manage game rooms
type RoomManager struct {
	rooms map[uint64]*room.Room
	wg    sync.WaitGroup
	rw    sync.RWMutex
}

// NewRoomManager creates a new room manager
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[uint64]*room.Room),
	}
}

// CreateRoom creates a game room
func (rm *RoomManager) CreateRoom(
	rid uint64, typeID int32, pid []uint64, randomSeed int32, logicServer string,
) (*room.Room, error) {
	rm.rw.Lock()
	defer rm.rw.Unlock()

	r, ok := rm.rooms[rid]
	if ok {
		return nil, fmt.Errorf("room id[%d] exists", rid)
	}

	r = room.NewRoom(rid, typeID, pid, randomSeed, logicServer)
	rm.rooms[rid] = r

	go func() {
		rm.wg.Add(1)
		defer func() {
			defer rm.wg.Done()
			rm.rw.Lock()
			delete(rm.rooms, rid)
			rm.rw.Unlock()
		}()
		r.Run()
	}()

	return r, nil
}

// GetRoom gets the specific room
func (rm *RoomManager) GetRoom(id uint64) *room.Room {
	rm.rw.RLock()
	defer rm.rw.RUnlock()

	r, _ := rm.rooms[id]
	return r
}

// RoomNum gets the count of the room
func (rm *RoomManager) RoomNum() int {
	rm.rw.RLock()
	defer rm.rw.RUnlock()

	return len(rm.rooms)
}

// Stop stops the room
func (rm *RoomManager) Stop() {
	rm.rw.Lock()
	for _, r := range rm.rooms {
		r.Stop()
	}
	rm.rooms = make(map[uint64]*room.Room)
	rm.rw.Unlock()

	rm.wg.Wait()
}
