NAME=kubevirt
BINARY=packer-plugin-${NAME}

COUNT?=1
TEST?=$(shell go list ./...)
HASHICORP_PACKER_PLUGIN_SDK_VERSION?=$(shell go list -m github.com/hashicorp/packer-plugin-sdk | cut -d " " -f2)

export KUBECONFIG=$(shell pwd)/kubeconfig
KUBEVIRT_VERSION=v0.49.0
CDI_VERSION=v1.43.1
IMAGES=quay.io/kubevirt/cdi-apiserver:$(CDI_VERSION)\
 quay.io/kubevirt/cdi-controller:$(CDI_VERSION)\
 quay.io/kubevirt/cdi-importer:$(CDI_VERSION)\
 quay.io/kubevirt/cdi-operator:$(CDI_VERSION)\
 quay.io/kubevirt/cdi-uploadproxy:$(CDI_VERSION)\
 quay.io/kubevirt/virt-api:$(KUBEVIRT_VERSION)\
 quay.io/kubevirt/virt-controller:$(KUBEVIRT_VERSION)\
 quay.io/kubevirt/virt-handler:$(KUBEVIRT_VERSION)\
 quay.io/kubevirt/virt-launcher:$(KUBEVIRT_VERSION)\
 quay.io/kubevirt/virt-operator:$(KUBEVIRT_VERSION)\
 quay.io/kubevirt/fedora-cloud-container-disk-demo:v0.36.5

.PHONY: dev

build:
	@go build -o ${BINARY}

dev: build
	@mkdir -p ~/.packer.d/plugins/
	@mv ${BINARY} ~/.packer.d/plugins/${BINARY}

test:
	@go test -race -count $(COUNT) $(TEST) -timeout=3m

install-packer-sdc:
	@go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@${HASHICORP_PACKER_PLUGIN_SDK_VERSION}

ci-release-docs: install-packer-sdc
	@packer-sdc renderdocs -src docs -partials docs-partials/ -dst docs/
	@/bin/sh -c "[ -d docs ] && zip -r docs.zip docs/"

plugin-check: install-packer-sdc build
	@packer-sdc plugin-check ${BINARY}

testacc: dev $(KUBECONFIG)
	@kubectl delete dv example || true
	@PACKER_ACC=1 go test -count $(COUNT) -v $(TEST) -timeout=120m

$(KUBECONFIG):
	@kind create cluster --name $(NAME) --wait 30m
	@$(foreach var,$(IMAGES),docker pull $(var);)
	@$(foreach var,$(IMAGES),kind load docker-image --name $(NAME) $(var);)
	@kubectl create -f https://github.com/kubevirt/kubevirt/releases/download/$(KUBEVIRT_VERSION)/kubevirt-operator.yaml
	@kubectl create -f https://github.com/kubevirt/kubevirt/releases/download/$(KUBEVIRT_VERSION)/kubevirt-cr.yaml
	@kubectl create -f https://github.com/kubevirt/containerized-data-importer/releases/download/$(CDI_VERSION)/cdi-operator.yaml
	@kubectl create -f https://github.com/kubevirt/containerized-data-importer/releases/download/$(CDI_VERSION)/cdi-cr.yaml
	@kubectl -n kubevirt wait --for=condition=Available --timeout=30m kubevirt/kubevirt
	@kubectl -n cdi wait --for=condition=Available --timeout=30m cdi/cdi

teardown:
	@kind delete cluster --name $(NAME)
	@rm $(KUBECONFIG)

generate: install-packer-sdc
	@go generate ./...
	@packer-sdc renderdocs -src ./docs -dst ./.docs -partials ./docs-partials
