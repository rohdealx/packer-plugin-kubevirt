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
  ssh_username = "fedora"
  ssh_password = "fedora"

  container_disk {
    image = "quay.io/kubevirt/fedora-cloud-container-disk-demo:v0.36.5"
    disk {
      boot_order = 1
    }
  }

  cloud_init {
    files = {
      userdata = local.user_data,
    }
  }

  data_volume {
    name        = "example"
    size        = "5Gi"
    source_type = "blank"
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
    expect_disconnect = true
    skip_clean        = true
    inline = [
      "sudo shutdown -h now",
    ]
  }
}
