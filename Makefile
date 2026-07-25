##
# goimapnotify
#
# @file
# @version 0.1

# Definir las variables para la información de Git
GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_TAG := $(shell git describe --tags)
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

# Definir las flags del linker
LDFLAGS := -X github.com/dsh2dsh/goimapnotify/internal/cli.commit=$(GIT_COMMIT) \
	-X github.com/dsh2dsh/goimapnotify/internal/cli.gittag=$(GIT_TAG) \
	-X github.com/dsh2dsh/goimapnotify/internal/cli.branch=$(GIT_BRANCH)

build:
	go build -ldflags "$(LDFLAGS)" -gcflags  '-N -l' ./

changelog:
	git-chglog -o CHANGELOG.md 2.3.14..

# end
