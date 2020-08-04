VERSION = 0.0.1
IMAGE = cyclops:$(VERSION)

MANAGER_BIN = cyclops
CLI_BIN = kubectl-cycle
OBSERVER_BIN = observer

.PHONY: build-manager build-observer build-cli install-cli build docker build-manager-linux build-observer-linux build-cli-linux build-linux docker-save local srcclr
.DEFAULT_GOAL := build

install-cli:
	go build -o ${GOPATH}/bin/${CLI_BIN} -ldflags="-X main.version=${VERSION}" cmd/cli/main.go

build-observer:
	go build -o bin/${OBSERVER_BIN} -ldflags="-X main.version=${VERSION}" cmd/observer/main.go

build-manager:
	go build -o bin/${MANAGER_BIN} -ldflags="-X main.version=${VERSION}" cmd/manager/main.go

build-cli:
	go build -o bin/${CLI_BIN} -ldflags="-X main.version=${VERSION}" cmd/cli/main.go

build: build-manager build-cli build-observer

build-manager-linux:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/linux/${MANAGER_BIN} -ldflags="-X main.version=${VERSION}" cmd/manager/main.go

build-cli-linux:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/linux/${CLI_BIN} -ldflags="-X main.version=${VERSION}" cmd/cli/main.go

build-observer-linux:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/linux/${OBSERVER_BIN} -ldflags="-X main.version=${VERSION}" cmd/observer/main.go

build-linux: build-manager-linux build-cli-linux build-observer-linux

clean:
	rm -f bin/${MANAGER_BIN}
	rm -f bin/${CLI_BIN}
	rm -f bin/${OBSERVER_BIN}
	rm -f bin/linux/${MANAGER_BIN}
	rm -f bin/linux/${CLI_BIN}
	rm -f bin/linux/${OBSERVER_BIN}


test:
	go test -cover ./pkg/...
	go test -cover ./cmd/...

check:
	golint -set_exit_status ./cmd/...
	golint -set_exit_status ./pkg/cloudprovider/...
	golint -set_exit_status ./pkg/controller/...
	golint -set_exit_status ./pkg/k8s/...
	golint -set_exit_status ./pkg/metrics/...
	golint -set_exit_status ./pkg/cli/...
	golint -set_exit_status ./pkg/generation/...
	golint -set_exit_status ./pkg/observer/...

docker:
	docker build -t $(IMAGE) .

install-operator-sdk:
	mkdir -p $(GOPATH)/src/github.com/operator-framework
	-cd $(GOPATH)/src/github.com/operator-framework && git clone https://github.com/operator-framework/operator-sdk
	git -C $(GOPATH)/src/github.com/operator-framework/operator-sdk checkout master
	$(MAKE) -C $(GOPATH)/src/github.com/operator-framework/operator-sdk tidy
	$(MAKE) -C $(GOPATH)/src/github.com/operator-framework/operator-sdk install

# See https://github.com/operator-framework/operator-sdk/blob/master/doc/user-guide.md
generate-crds:
	mkdir -p build deploy/crds
	touch build/Dockerfile
	operator-sdk generate k8s
	operator-sdk generate crds
	rm -rf build/