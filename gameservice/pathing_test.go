package gameservice

import (
	"gw1/server/pathing"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstanceAttachesPathData(t *testing.T) {
	old := instancePathStore
	defer func() { instancePathStore = old }()

	sd := &pathing.PathData{
		Planes: []pathing.Plane{{PlaneID: 0}},
	}
	store := pathing.NewStore()
	store.Set(0x1b97d, sd) // Lakeside County's map file id
	instancePathStore = store

	inst := newHeadlessInstance(t, 146)
	assert.Same(t, sd, inst.path, "instance should attach the navmesh for its map file id")

	_, err := newInstance(3, instanceDefinitions.Instances[3])
	require.Error(t, err, "an instance must not be created without pathing data")
	assert.ErrorContains(t, err, "no pathing data")
}
func TestNewInstanceWithoutStoreFails(t *testing.T) {
	old := instancePathStore
	defer func() { instancePathStore = old }()
	instancePathStore = nil

	_, err := newInstance(146, instanceDefinitions.Instances[146])
	require.Error(t, err)
	assert.ErrorContains(t, err, "pathing store is not initialized")
}

func TestNewInstanceFailsWhenMapFileUnreadable(t *testing.T) {
	old := instancePathStore
	defer func() { instancePathStore = old }()

	store := pathing.NewStore()
	instancePathStore = store

	_, err := newInstance(3, instanceDefinitions.Instances[3])
	require.Error(t, err)
	assert.ErrorContains(t, err, "no pathing data for map")
}

func TestInitializeInstancesFailsWithoutArchive(t *testing.T) {
	err := InitializeInstances("/nonexistent/Gw.dat")
	require.Error(t, err)
}
