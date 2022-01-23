//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,DataVolumeConfig,DataVolumeSourceConfig,DiskConfig

package main

import (
	"errors"
	"os"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"k8s.io/apimachinery/pkg/api/resource"
)

const BuilderId = "rohdealx.kubevirt"

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	KubeConfigPath string `mapstructure:"kube_config_path"`

	SSHPort              int           `mapstructure:"ssh_port"`
	SSHTimeout           time.Duration `mapstructure:"ssh_timeout"`
	SSHKeepAliveInterval time.Duration `mapstructure:"ssh_keep_alive_interval"`
	SSHHandshakeAttempts int           `mapstructure:"ssh_handshake_attempts"`
	SSHUsername          string        `mapstructure:"ssh_username"`
	SSHPassword          string        `mapstructure:"ssh_password"`

	Namespace string `mapstructure:"namespace"`
	Name      string `mapstructure:"name"`

	EFI        bool `mapstructure:"efi"`
	SecureBoot bool `mapstructure:"secure_boot"`

	CPU    string `mapstructure:"cpu"`
	Memory string `mapstructure:"memory"`
	// The hugepage size, for x86_64 architecture valid values are 1Gi and 2Mi.
	HugepagesPageSize string   `mapstructure:"hugepages_page_size" required:"false"`
	GPUs              []string `mapstructure:"gpus"`

	DataVolume DataVolumeConfig `mapstructure:"data_volume"`
	Disks      []DiskConfig     `mapstructure:"disk"`

	ctx interpolate.Context
}

type DataVolumeConfig struct {
	Size             string                 `mapstructure:"size"`
	VolumeMode       string                 `mapstructure:"volume_mode"`
	StorageClassName string                 `mapstructure:"storage_class_name"`
	Preallocation    bool                   `mapstructure:"preallocation"`
	Source           DataVolumeSourceConfig `mapstructure:"source"`
}

type DataVolumeSourceConfig struct {
	Type string `mapstructure:"type"`
	URL  string `mapstructure:"url"`
}

type DiskConfig struct {
	Name     string            `mapstructure:"name"`
	Type     string            `mapstructure:"type"`
	DiskType string            `mapstructure:"disk_type"`
	Image    string            `mapstructure:"image"`
	Files    map[string]string `mapstructure:"files"`
}

func (c *Config) Prepare(raws ...interface{}) ([]string, []string, error) {
	opts := config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &c.ctx,
	}
	err := config.Decode(c, &opts, raws...)
	if err != nil {
		return nil, nil, err
	}
	var errs *packer.MultiError
	packer.LogSecretFilter.Set(c.SSHPassword)

	if c.KubeConfigPath == "" {
		c.KubeConfigPath = os.Getenv("KUBECONFIG")
	}

	if c.SSHPort == 0 {
		c.SSHPort = 22
	}
	if c.SSHTimeout == 0 {
		c.SSHTimeout = 60 * time.Minute
	}
	if c.SSHKeepAliveInterval == 0 {
		c.SSHKeepAliveInterval = 5 * time.Second
	}
	if c.SSHHandshakeAttempts == 0 {
		c.SSHHandshakeAttempts = 20
	}
	if c.SSHUsername == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("ssh_username must be specified"))
	}
	if c.SSHPassword == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("ssh_password must be specified"))
	}

	if c.Namespace == "" {
		c.Namespace = "default"
	}
	if c.Name == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("name must be specified"))
	}

	if c.SecureBoot {
		c.EFI = true
	}

	if c.CPU == "" {
		c.CPU = "4"
	} else {
		_, err := resource.ParseQuantity(c.CPU)
		if err != nil {
			errs = packer.MultiErrorAppend(errs, err)
		}
	}
	if c.Memory == "" {
		c.Memory = "4Gi"
	} else {
		_, err := resource.ParseQuantity(c.Memory)
		if err != nil {
			errs = packer.MultiErrorAppend(errs, err)
		}
	}

	// TODO DataVolume
	// TODO Disks

	if errs != nil && len(errs.Errors) > 0 {
		return nil, nil, errs
	}
	return nil, nil, nil
}
