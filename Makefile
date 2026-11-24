include config/versions.env
export

CACHE_GO_MOD   ?= $(HOME)/.cache/go-mod
CACHE_GO_BUILD ?= $(HOME)/.cache/go-build

.PHONY: fetch test vet build image smoke clean

fetch:
	./scripts/fetch.sh

vet:
	podman run --rm --network=none \
		-v $(CURDIR):/src:ro,z \
		-v $(CACHE_GO_MOD):/go/pkg/mod:z \
		-v $(CACHE_GO_BUILD):/root/.cache/go-build:z \
		-w /src $(GO_IMAGE) \
		go vet ./...

test:
	podman run --rm --network=none \
		-v $(CURDIR):/src:ro,z \
		-v $(CACHE_GO_MOD):/go/pkg/mod:z \
		-v $(CACHE_GO_BUILD):/root/.cache/go-build:z \
		-w /src $(GO_IMAGE) \
		go test -v ./...

build:
	./scripts/build-binary.sh build/gateway

image: build
	./scripts/build-image.sh config/ops.smoke.json gateway-smoke

smoke: image
	./scripts/smoke-qemu.sh build/gateway-smoke.img

clean:
	rm -rf build/
