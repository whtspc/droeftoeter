BINARY = droeftoeter
VERSION ?= dev
LDFLAGS = -s -w
DIST = dist

PLATFORMS = \
	windows/amd64 \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64

.PHONY: all clean $(PLATFORMS)

all: $(PLATFORMS)

$(PLATFORMS):
	$(eval GOOS := $(word 1,$(subst /, ,$@)))
	$(eval GOARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT := $(if $(filter windows,$(GOOS)),.exe,))
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-$(GOOS)-$(GOARCH)$(EXT) .

clean:
	rm -rf $(DIST)
