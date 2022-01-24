package main

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func (c ContainerDiskConfig) GetName() string {
	return fmt.Sprintf("containerdisk-%d", c.id)
}

func (c ContainerDiskConfig) GetDiskConfig() DiskConfig {
	return c.Disk
}

func (c ContainerDiskConfig) GetVolume(multistep.StateBag) (kubevirtv1.Volume, error) {
	volume := kubevirtv1.Volume{
		Name: c.GetName(),
		VolumeSource: kubevirtv1.VolumeSource{
			ContainerDisk: &kubevirtv1.ContainerDiskSource{
				Image: c.Image,
			},
		},
	}
	return volume, nil
}
