# Standard variables defining directories and other useful stuff.
PROJECT_WORKSPACE	?= $(CURDIR)
INCLUDE_BUILD_DIR	?= $(PROJECT_WORKSPACE)/build
PROJECT_NAME		:= newrelic-infra
TARGET_DIR			= $(PROJECT_WORKSPACE)/target
TARGET_DIR_CENTOS5	= $(TARGET_DIR)/el_5
UNAME_S				:= $(shell uname -s)

# Default GO_BIN to Go binary in PATH
GO_BIN				?= go

# Centos 5 cannot use a Go version greater than 1.9
GO_BIN_1_9			?= go1.9.4

# Scripts for building the Agent
include $(INCLUDE_BUILD_DIR)/infra_build.mk

# Scripts for getting On Host Integrations
# https://docs.newrelic.com/docs/integrations/host-integrations/getting-started/introduction-host-integrations
include $(INCLUDE_BUILD_DIR)/embed_ohis.mk

include $(INCLUDE_BUILD_DIR)/embed_fb.mk

# Scripts for CICD
include $(INCLUDE_BUILD_DIR)/ci.mk

include $(INCLUDE_BUILD_DIR)/release.mk
