package main

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func (c SysprepConfig) GetName() string {
	return fmt.Sprintf("sysprep-%d", c.id)
}

func (c SysprepConfig) GetDiskConfig() DiskConfig {
	return c.Disk
}

func (c SysprepConfig) GetVolume(state multistep.StateBag) (kubevirtv1.Volume, error) {
	names := state.Get(SysprepNames).([]string)
	volume := kubevirtv1.Volume{
		Name: c.GetName(),
		VolumeSource: kubevirtv1.VolumeSource{
			Sysprep: &kubevirtv1.SysprepSource{
				Secret: &corev1.LocalObjectReference{
					Name: names[c.id],
				},
			},
		},
	}
	return volume, nil
}
