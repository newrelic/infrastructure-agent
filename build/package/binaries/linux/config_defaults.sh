#!/bin/bash
#
#   This is the default method to configure the Infrastructure agent installation.
#   However, you can override this config by setting environment variables
#   which is ideal for containerized environments.
#

#
#   Specify where the agent binary will be located.
#   Corresponding env var: '$NRIA_BIN_DIR'
#
bin_dir="/usr/local/bin"

#
#   The Linux agent can run as root, privileged, or unprivileged user.
#   Set this value: ROOT, PRIVILEGED or UNPRIVILEGED
#   read more: (https://docs.newrelic.com/docs/infrastructure/new-relic-infrastructure/installation/install-infrastructure-linux#agent-mode-intro)
#   Corresponding env var: '$NRIA_MODE'
#
mode="ROOT"

#
#   Set the user that will run the agent binary, required only if mode/NRIA_MODE 
#   is set to PRIVILEGED or UNPRIVILEGED
#   Corresponding env var: '$NRIA_USER'
#
#user=""

#
#   Agent configuration file location.
#   Corresponding env var: '$NRIA_CONFIG_FILE'
#
config_file="/etc/newrelic-infra.yml"

#
#   Agent pid file.
#   Corresponding env var: '$NRIA_PID_FILE'
#
pid_file="/var/run/newrelic-infra/newrelic-infra.pid"

#
#   Agent home directory.
#   Corresponding env var: '$NRIA_AGENT_DIR'
#
agent_dir="/var/db/newrelic-infra/"

#
#   Directory containing agent integration configuration files.
#   Corresponding env var: '$NRIA_PLUGIN_DIR'
#
plugin_dir="/etc/newrelic-infra/integrations.d/"

#
#   Agent log file location.
#   Corresponding env var: '$NRIA_LOG_FILE'
#
log_file="/var/log/newrelic-infra/newrelic-infra.log"

#
#   New Relic Infrastructure license key.
#   Corresponding env var: '$NRIA_LICENSE_KEY'
#
#license_key=""
