# Standard variables defining directories and other useful stuff.
PROJECT_WORKSPACE	?= $(CURDIR)
INCLUDE_BUILD_DIR	?= $(PROJECT_WORKSPACE)/build
PROJECT_NAME		:= newrelic-infra
TARGET_DIR			?= $(PROJECT_WORKSPACE)/target
DIST_DIR			?= $(PROJECT_WORKSPACE)/dist
TARGET_DIR_CENTOS5	= $(TARGET_DIR)/el_5
UNAME_S				:= $(shell uname -s)
COVERAGE_FILE       ?= coverage.out

# Default GO_BIN to Go binary in PATH
GO_BIN				?= go

# Centos 5 cannot use a Go version greater than 1.9
GO_BIN_1_9			?= go1.9.4

# Scripts for building the Agent
include $(INCLUDE_BUILD_DIR)/build.mk

# Scripts for getting On Host Integrations
# https://docs.newrelic.com/docs/integrations/host-integrations/getting-started/introduction-host-integrations
include $(INCLUDE_BUILD_DIR)/embed/integrations.mk

include $(INCLUDE_BUILD_DIR)/embed/fluent-bit.mk

# Scripts for CICD
include $(INCLUDE_BUILD_DIR)/ci.mk

include $(INCLUDE_BUILD_DIR)/release.mk
