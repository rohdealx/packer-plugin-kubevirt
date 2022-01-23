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

	// Path to the kubeconfig file. Can also be set via the `KUBECONFIG` environment variable.
	KubeConfigPath string `mapstructure:"kube_config_path"`

	// The port to connect to ssh. This defaults to `22`.
	SSHPort int `mapstructure:"ssh_port"`
	// The time to wait for ssh to become available. Packer uses this to
	// determine when the machine has booted so this is usually quite long.
	// Example value: `10m`.
	SSHTimeout time.Duration `mapstructure:"ssh_timeout"`
	// How often to send "keep alive" messages to the server.
	// Example value: `10s`. Defaults to `5s`.
	SSHKeepAliveInterval time.Duration `mapstructure:"ssh_keep_alive_interval"`
	// The number of handshakes to attempt with ssh once it can connect. This
	// defaults to `10`.
	SSHHandshakeAttempts int `mapstructure:"ssh_handshake_attempts"`
	// The username used to authenticate.
	SSHUsername string `mapstructure:"ssh_username" required:"true"`
	// The plaintext password used to authenticate.
	SSHPassword string `mapstructure:"ssh_password" required:"true"`

	Namespace  string           `mapstructure:"namespace"`
	Name       string           `mapstructure:"name"`
	DataVolume DataVolumeConfig `mapstructure:"data_volume"`

	// If `true`, efi will be used instead of bios.
	EFI bool `mapstructure:"efi"`
	// Implies [`efi`](#efi) `true`.
	SecureBoot bool `mapstructure:"secure_boot"`

	// In [Kubernetes cpu resource units](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-cpu).
	CPU string `mapstructure:"cpu"`
	// In [Kubernetes memory resource units](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory).
	Memory string `mapstructure:"memory"`
	// The hugepage size, for x86_64 architecture valid values are 1Gi and 2Mi.
	HugepagesPageSize string `mapstructure:"hugepages_page_size" required:"false"`
	// List of gpus device names e.g. `nvidia.com/TU104GL_Tesla_T4`.
	GPUs []string `mapstructure:"gpus"`

	// There has to be at least one disk,
	// this first disk has to be of type `datavolume` and is the resulting artifact.
	Disks []DiskConfig `mapstructure:"disk"`

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
		c.SSHHandshakeAttempts = 10
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

	// TODO Disks

	if errs != nil && len(errs.Errors) > 0 {
		return nil, nil, errs
	}
	return nil, nil, nil
}
