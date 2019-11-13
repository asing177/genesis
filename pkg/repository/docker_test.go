/*
	Copyright 2019 whiteblock Inc.
	This file is a part of the genesis.

	Genesis is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	Genesis is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package repository

import (
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	entityMock "github.com/whiteblock/genesis/mocks/pkg/entity"
)

func TestDockerRepository_ContainerCreate(t *testing.T) {
	//todo
}

func TestDockerRepository_ContainerList(t *testing.T) {
	//todo
}

func TestDockerRepository_ContainerRemove(t *testing.T) {
	//todo
}

func TestDockerRepository_ContainerStart(t *testing.T) {
	//todo
}

func TestDockerRepository_ImageLoad(t *testing.T) {
	//todo
}

func TestDockerRepository_ImagePull(t *testing.T) {
	//todo
}

func TestDockerRepository_ImageList(t *testing.T) {
	//todo
}

func TestDockerRepository_NetworkConnect(t *testing.T) {
	//todo
}

func TestDockerRepository_NetworkCreate(t *testing.T) {
	//todo
}

func TestDockerRepository_NetworkDisconnect(t *testing.T) {
	//todo
}

func TestDockerRepository_NetworkRemove(t *testing.T) {
	//todo
}

func TestDockerRepository_NetworkList(t *testing.T) {
	//todo
}

func TestDockerRepository_VolumeList(t *testing.T) {
	cli := new(entityMock.Client)
	testFilters := filters.Args{}
	result := volume.VolumeListOKBody{}

	cli.On("VolumeList", mock.Anything, mock.Anything).Return(result, nil).Run(func(args mock.Arguments) {
		require.Len(t, args, 2)
		assert.Nil(t, args.Get(0))
		assert.Equal(t, testFilters, args.Get(1))
	})

	repo := NewDockerRepository()
	res, err := repo.VolumeList(nil, cli, testFilters)
	assert.NoError(t, err)
	assert.Equal(t, result, res)

	cli.AssertExpectations(t)
}

func TestDockerRepository_VolumeRemove(t *testing.T) {
	cli := new(entityMock.Client)
	volumeID := "volume1"
	isForced := true

	cli.On("VolumeRemove", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		require.Len(t, args, 3)
		assert.Nil(t, args.Get(0))
		assert.Equal(t, volumeID, args.String(1))
		assert.Equal(t, isForced, args.Bool(2))
	}).Once()

	repo := NewDockerRepository()

	err := repo.VolumeRemove(nil, cli, volumeID, isForced)
	assert.NoError(t, err)
	cli.AssertExpectations(t)
}

func TestDockerRepository_VolumeCreate(t *testing.T) { //todo why isn't this one working?
	cli := new(entityMock.Client)
	options := volume.VolumeCreateBody{
		Name:   "test_volume",
		Labels: map[string]string{"foo": "bar"},
	}

	expectedVol := types.Volume{
		Name: options.Name,
		Labels: options.Labels,
	}

	cli.On("VolumeCreate", mock.Anything, mock.Anything).Return(expectedVol, nil).Run(func(args mock.Arguments) {
		require.Len(t, args, 2)
		assert.Nil(t, args.Get(0))
		assert.ElementsMatch(t, options, args.Get(1))
	}).Once()

	repo := NewDockerRepository()

	vol, err := repo.VolumeCreate(nil, cli, options)
	assert.NoError(t, err)

	fmt.Println(vol)
	fmt.Println(expectedVol)

	assert.ElementsMatch(t, expectedVol, vol)
}
