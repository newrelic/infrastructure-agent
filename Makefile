# Standard variables defining directories and other useful stuff.
PROJECT_WORKSPACE	:= $(CURDIR)
PROJECT_NAME		:= newrelic-infra
TARGET_DIR			= $(PROJECT_WORKSPACE)/target
TARGET_DIR_CENTOS5	= $(TARGET_DIR)/el_5
UNAME_S				:= $(shell uname -s)

# Default GO_BIN to Go binary in PATH
GO_BIN				?= go

# Centos 5 cannot use a Go version greater than 1.9
GO_BIN_1_9			?= go1.9.4

# Scripts for building the Agent
include scripts/infra_build.mk
