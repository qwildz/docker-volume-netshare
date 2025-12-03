VERSION = 0.36
GO_FMT = gofmt -s -w -l .
GO_XC = goxc -os="linux" -bc="linux,amd64,arm" -tasks-="rmbin"
BUILD_DATE = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -ldflags="-s -w -X main.VERSION=$(VERSION) -X main.BUILD_DATE=$(BUILD_DATE)"

GOXC_FILE = .goxc.local.json

all: deps compile

compile: goxc

# Build binary for current platform
build:
	go build $(LDFLAGS) -o docker-volume-netshare .

# Build .deb packages for Debian/Ubuntu (requires nfpm)
deb:
	cd packaging && ./build-deb.sh $(VERSION)

# Build binary for all platforms
build-all:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o build/docker-volume-netshare_linux_amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o build/docker-volume-netshare_linux_arm64 .

goxc:
	$(shell echo '{\n "ConfigVersion": "0.9",\n "PackageVersion": "$(VERSION)",' > $(GOXC_FILE))
	$(shell echo ' "TaskSettings": {' >> $(GOXC_FILE))
	$(shell echo '  "bintray": {\n   "apikey": "$(BINTRAY_APIKEY)"' >> $(GOXC_FILE))
	$(shell echo '  },' >> $(GOXC_FILE))
	$(shell echo '  "publish-github": {' >> $(GOXC_FILE))
	$(shell echo '     "apikey": "$(GITHUB_APIKEY)",' >> $(GOXC_FILE))
	$(shell echo '     "body": "",' >> $(GOXC_FILE))
	$(shell echo '     "include": "*.zip,*.tar.gz,*.deb,docker-volume-netshare_$(VERSION)_linux_amd64-bin,docker-volume-netshare_$(VERSION)_linux_arm-bin"' >> $(GOXC_FILE))
	$(shell echo '  }\n } \n}' >> $(GOXC_FILE))
	$(GO_XC)
	cp build/$(VERSION)/linux_amd64/docker-volume-netshare build/$(VERSION)/docker-volume-netshare_$(VERSION)_linux_amd64-bin
	cp build/$(VERSION)/linux_arm/docker-volume-netshare build/$(VERSION)/docker-volume-netshare_$(VERSION)_linux_arm-bin

deps:
	go mod download

format:
	$(GO_FMT)

clean:
	rm -rf build/ docker-volume-netshare

bintray:
	$(GO_XC) bintray

github:
	$(GO_XC) publish-github

.PHONY: all compile build build-all deb goxc deps format clean bintray github
