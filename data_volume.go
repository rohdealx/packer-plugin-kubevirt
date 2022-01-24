package main

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func (c DataVolumeConfig) GetName() string {
	return fmt.Sprintf("datavolume-%d", c.id)
}

func (c DataVolumeConfig) GetDiskConfig() DiskConfig {
	return c.Disk
}

func (c DataVolumeConfig) GetVolume(state multistep.StateBag) (kubevirtv1.Volume, error) {
	names := state.Get(DataVolumeNames).([]string)
	volume := kubevirtv1.Volume{
		Name: c.GetName(),
		VolumeSource: kubevirtv1.VolumeSource{
			DataVolume: &kubevirtv1.DataVolumeSource{
				Name: names[c.id],
			},
		},
	}
	return volume, nil
}
