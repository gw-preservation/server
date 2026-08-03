package gameservice

import (
	"fmt"
	"gw1/server/pathing"
	"sync"
)

var instancePathStore *pathing.Store

var pathSummaryLogged = struct {
	sync.Mutex
	ids map[uint32]struct{}
}{ids: make(map[uint32]struct{})}

func initializePathing(gwdatPath string) error {
	store, err := pathing.NewLazyStore(gwdatPath)
	if err != nil {
		return err
	}
	instancePathStore = store
	log.Info().Str("gwdat", gwdatPath).Msg("data archive opened")
	return nil
}

// ensurePathLoaded returns the navmesh for a map file id, loading it on demand.
func ensurePathLoaded(fileID uint32, name string) (*pathing.PathData, error) {
	if instancePathStore == nil {
		return nil, fmt.Errorf("pathing store is not initialized")
	}
	sd, err := instancePathStore.EnsureLoaded(fileID)
	if err != nil {
		return nil, fmt.Errorf("no pathing data for map %q (file_id 0x%08x): %w", name, fileID, err)
	}
	pathSummaryLogged.Lock()
	_, logged := pathSummaryLogged.ids[fileID]
	if !logged {
		pathSummaryLogged.ids[fileID] = struct{}{}
	}
	pathSummaryLogged.Unlock()
	if !logged {
		logPathingSummary(fileID, name, sd)
	}
	return sd, nil
}

func logPathingSummary(fileID uint32, name string, sd *pathing.PathData) {
	var traps, xnodes, ynodes, sinks, portals int
	for _, plane := range sd.Planes {
		traps += len(plane.Trapezoids)
		portals += len(plane.Portals)
		for _, n := range plane.Nodes {
			switch n.Type {
			case pathing.NodeTypeXNode:
				xnodes++
			case pathing.NodeTypeYNode:
				ynodes++
			case pathing.NodeTypeSink:
				sinks++
			}
		}
	}
	log.Debug().
		Str("file_id", fmt.Sprintf("0x%08x", fileID)).
		Str("map", name).
		Int("planes", len(sd.Planes)).
		Int("traps", traps).
		Int("xnodes", xnodes).
		Int("ynodes", ynodes).
		Int("sinks", sinks).
		Int("portals", portals).
		Msg("loaded pathing data")
}
