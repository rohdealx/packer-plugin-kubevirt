//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config,DiskConfig,DataVolumeConfig,ContainerDiskConfig,CloudInitConfig,SysprepConfig

package main

import (
	"errors"
	"os"

	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	// Path to the kubeconfig file. Can also be set via the `KUBECONFIG` environment variable.
	KubeConfigPath string `mapstructure:"kube_config_path"`
	Namespace      string `mapstructure:"namespace"`

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

	DataVolumes    []DataVolumeConfig    `mapstructure:"data_volume"`
	ContainerDisks []ContainerDiskConfig `mapstructure:"container_disk"`
	CloudInits     []CloudInitConfig     `mapstructure:"cloud_init"`
	Syspreps       []SysprepConfig       `mapstructure:"sysprep"`

	// Runtime
	disks []Disk

	ctx interpolate.Context
}

type DataVolumeConfig struct {
	Disk DiskConfig `mapstructure:"disk" required:"false"`

	// if name is set export this data volume as an artifact and don't delete it
	Name string `mapstructure:"name" required:"false"`
	// Whether this data volume should be preallocated.
	Preallocation bool `mapstructure:"preallocation" required:"false"`

	// data volume source
	SourceType string `mapstructure:"source_type"`
	SourceURL  string `mapstructure:"source_url"`

	// persistent volume claim
	VolumeMode       string `mapstructure:"volume_mode" required:"false"`
	StorageClassName string `mapstructure:"storage_class_name" required:"false"`
	Size             string `mapstructure:"size"`

	id int
}

type ContainerDiskConfig struct {
	Disk  DiskConfig `mapstructure:"disk" required:"false"`
	Image string     `mapstructure:"image" required:"true"`

	id int
}

type CloudInitConfig struct {
	Disk  DiskConfig        `mapstructure:"disk" required:"false"`
	Files map[string]string `mapstructure:"files" required:"true"`

	id int
}

type SysprepConfig struct {
	Disk  DiskConfig        `mapstructure:"disk" required:"false"`
	Files map[string]string `mapstructure:"files" required:"true"`

	id int
}

type DiskConfig struct {
	// disk, cdrom
	Type string `mapstructure:"type" required:"false"`
	// value > 0, lower first
	BootOrder uint `mapstructure:"boot_order" required:"false"`
	// virtio, sata, scsi
	Bus string `mapstructure:"bus" required:"false"`
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
		if c.KubeConfigPath == "" {
			errs = packer.MultiErrorAppend(errs, errors.New("kube_config_path must be specified"))
		}
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

	for i, e := range c.DataVolumes {
		e.id = i
		c.disks = append(c.disks, e)
	}
	for i, e := range c.CloudInits {
		e.id = i
		c.disks = append(c.disks, e)
	}
	for i, e := range c.Syspreps {
		e.id = i
		c.disks = append(c.disks, e)
	}
	for i, e := range c.ContainerDisks {
		e.id = i
		c.disks = append(c.disks, e)
	}

	if errs != nil && len(errs.Errors) > 0 {
		return nil, nil, errs
	}
	return nil, nil, nil
}

type Disk interface {
	GetName() string
	GetVolume(multistep.StateBag) (kubevirtv1.Volume, error)
	GetDiskConfig() DiskConfig
}
