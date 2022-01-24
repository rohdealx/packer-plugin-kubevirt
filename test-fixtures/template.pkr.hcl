locals {
  user_data = <<EOF
#cloud-config
password: fedora
chpasswd:
  expire: false
ssh_pwauth: true
hostname: example
EOF
}

source "kubevirt" "example" {
  name = "example"

  ssh_username = "fedora"
  ssh_password = "fedora"

  disk {
    type        = "datavolume"
    name        = "example"
    size        = "5Gi"
    source_type = "registry"
    source_url  = "docker://quay.io/kubevirt/fedora-cloud-container-disk-demo"
  }

  disk {
    type = "cloudinit"
    name = "cloudinit"
    files = {
      userdata = local.user_data,
    }
  }
}

build {
  sources = ["source.kubevirt.example"]

  provisioner "shell" {
    inline = [
      "echo hello world",
    ]
  }

  provisioner "shell" {
    inline = [
      "sudo shutdown -h 1",
    ]
  }
}
