package net

import (
	peer "gx/ipfs/QmXYjuNuxVzXKJCfWasQk1RqkhVLDM9jtUKhqc2WPQmFSB/go-libp2p-peer"
	"sync"
)

type BanManager struct {
	blockedIds map[string]bool
	*sync.RWMutex
}

func NewBanManager(blockedIds []peer.ID) *BanManager {
	blockedMap := make(map[string]bool)
	for _, pid := range blockedIds {
		blockedMap[pid.Pretty()] = true
	}
	return &BanManager{blockedMap, new(sync.RWMutex)}
}

func (bm *BanManager) AddBlockedId(peerId peer.ID) {
	bm.Lock()
	defer bm.Unlock()
	bm.blockedIds[peerId.Pretty()] = true
}

func (bm *BanManager) RemoveBlockedId(peerId peer.ID) {
	bm.Lock()
	defer bm.Unlock()
	if bm.blockedIds[peerId.Pretty()] {
		delete(bm.blockedIds, peerId.Pretty())
	}
}

func (bm *BanManager) SetBlockedIds(peerIds []peer.ID) {
	bm.Lock()
	defer bm.Unlock()

	bm.blockedIds = make(map[string]bool)

	for _, pid := range peerIds {
		bm.blockedIds[pid.Pretty()] = true
	}
}

func (bm *BanManager) GetBlockedIds() []peer.ID {
	bm.RLock()
	defer bm.RUnlock()
	var ret []peer.ID
	for pid := range bm.blockedIds {
		id, err := peer.IDB58Decode(pid)
		if err != nil {
			continue
		}
		ret = append(ret, id)
	}
	return ret
}

func (bm *BanManager) IsBanned(peerId peer.ID) bool {
	bm.RLock()
	defer bm.RUnlock()
	return bm.blockedIds[peerId.Pretty()]
}
