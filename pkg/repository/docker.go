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
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

//DockerRepository represents direct interacts with the docker daemon
type DockerRepository interface {
	//ContainerCreate creates a new container based in the given configuration. It can be associated with a name, but it's not mandatory.
	ContainerCreate(ctx context.Context, cli *client.Client, config *container.Config, hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error)

	//ContainerStart sends a request to the docker daemon to start a container.
	ContainerStart(ctx context.Context, cli *client.Client, containerID string, options types.ContainerStartOptions) error
}

type dockerRepository struct {
}

//NewDockerRepository creates a new DockerRepository
func NewDockerRepository() DockerRepository {
	return &dockerRepository{}
}

//ContainerCreate creates a new container based in the given configuration. It can be associated with a name, but it's not mandatory.
func (dr dockerRepository) ContainerCreate(ctx context.Context, cli *client.Client, config *container.Config,
	hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig,
	containerName string) (container.ContainerCreateCreatedBody, error) {
	return cli.ContainerCreate(ctx, config, hostConfig, networkingConfig, containerName)
}

//ContainerStart sends a request to the docker daemon to start a container.
func (dr dockerRepository) ContainerStart(ctx context.Context, cli *client.Client,
	containerID string, options types.ContainerStartOptions) error {
	return cli.ContainerStart(ctx, containerID, options)
}