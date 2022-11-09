#!/bin/bash

#
#   Installation script for installing New Relic Infrastructure agent.
#

# If the environment variables are not set, use the default values provided in config.sh file.
load_config() {
    source config_defaults.sh

    # Installation config: defaults when unset

    [ -z "${NRIA_BIN_DIR}" ] && NRIA_BIN_DIR="${bin_dir}"
    [ -z "${NRIA_USER}" ] && NRIA_USER="${user}"
    [ -z "${NRIA_MODE}" ] && NRIA_MODE="${mode}"
    [ -z "${NRIA_CONFIG_FILE}" ] && NRIA_CONFIG_FILE="${config_file}"

    # Startup config: defaults when unset

    [ -z "${NRIA_PID_FILE}" ] && NRIA_PID_FILE="${pid_file}"
    [ -z "${NRIA_AGENT_DIR}" ] && NRIA_AGENT_DIR="${agent_dir}"
    [ -z "${NRIA_PLUGIN_DIR}" ] && NRIA_PLUGIN_DIR="${plugin_dir}"
    [ -z "${NRIA_LOG_FILE}" ] && NRIA_LOG_FILE="${log_file}"
    [ -z "${NRIA_LICENSE_KEY}" ] && ! [ -z "${license_key}" ] && NRIA_LICENSE_KEY="${license_key}"
}

print_config() {
    echo "Using configuration..."
    echo ""
    echo "  \$NRIA_BIN_DIR=${NRIA_BIN_DIR}"
    echo "  \$NRIA_MODE=${NRIA_MODE}"
    echo "  \$NRIA_USER=${NRIA_USER}"
    echo "  \$NRIA_CONFIG_FILE=${NRIA_CONFIG_FILE}"
    echo "  \$NRIA_PID_FILE=${NRIA_PID_FILE}"
    echo "  \$NRIA_AGENT_DIR=${NRIA_AGENT_DIR}"
    echo "  \$NRIA_PLUGIN_DIR=${NRIA_PLUGIN_DIR}"
    echo "  \$NRIA_LOG_FILE=${NRIA_LOG_FILE}"
    echo ""
}

install_service() {
    binary="${NRIA_BIN_DIR}/newrelic-infra"
    service_binary="${binary}-service"
    config="-config=${NRIA_CONFIG_FILE}"
    agent="${service_binary} ${config}"

    # Detect the init system.
    if command -v systemctl >/dev/null 2>&1; then
        install_systemd_agent
    elif command -v initctl >/dev/null 2>&1; then
        install_upstart_agent
    elif command -v update-rc.d >/dev/null 2>&1; then
        install_sysv_agent
    else
        echo "Unable to detect the init system continue the installation manually"
        exit 2
    fi
}


validate() {
    # Installation config

    if [ -z "${NRIA_LICENSE_KEY}" ]; then
        echo "no license key provided"
        exit 1
    fi

    sudoUser=""
    if [ ! -z "${NRIA_USER}" ]; then
        sudoUser="sudo -u ${NRIA_USER} "
        if [ ! $(getent passwd ${NRIA_USER}) ] ; then
            echo "user ${NRIA_USER} does not exists"
            exit 1
        fi
    fi

    if ${sudoUser} [ ! -r "${NRIA_BIN_DIR}" ]; then
        echo "binary directory (NRIA_BIN_DIR) not readable: '${NRIA_BIN_DIR}'"
        exit 1
    fi
    if ${sudoUser} [ ! -x "${NRIA_BIN_DIR}/newrelic-infra" ]; then
        echo "binary (NRIA_BIN_DIR) not executable: '${NRIA_BIN_DIR}/newrelic-infra'"
        exit 1
    fi

    if ${sudoUser} [ ! -x "${NRIA_BIN_DIR}/newrelic-infra-ctl" ]; then
        echo "binary (NRIA_BIN_DIR) not executable: '${NRIA_BIN_DIR}/newrelic-infra-ctl'"
        exit 1
    fi

    if ${sudoUser} [ ! -x "${NRIA_BIN_DIR}/newrelic-infra-service" ]; then
        echo "binary (NRIA_BIN_DIR) not executable: '${NRIA_BIN_DIR}/newrelic-infra-service'"
        exit 1
    fi

    if [ "${NRIA_MODE}" != "ROOT" ] && [ "${NRIA_MODE}" != "PRIVILEGED" ] && [ "${NRIA_MODE}" != "UNPRIVILEGED" ]; then
        echo "invalid NRIA_MODE: '${NRIA_MODE}'"
        exit 1
    fi

    if ${sudoUser} [ ! -r "${NRIA_CONFIG_FILE}" ]; then
        echo "config file (NRIA_CONFIG_FILE) not readable: '${NRIA_CONFIG_FILE}'"
        exit 1
    fi


    # Startup config

    if ${sudoUser} [ -f "${NRIA_PID_FILE}" ] && ${sudoUser} [ ! -w "${NRIA_PID_FILE}" ]; then
        echo "pid file (NRIA_PID_FILE) not writable: ${NRIA_PID_FILE}"
        exit 1
    else
        pidDir=$(dirname "${NRIA_PID_FILE}")
        if ${sudoUser} [ ! -w "${pidDir}" ]; then
            echo "pid file directory not writable: ${pidDir}"
            exit 1
        fi
    fi

    if ${sudoUser} [ ! -w "${NRIA_AGENT_DIR}" ]; then
        echo "cache directory (NRIA_AGENT_DIR) not writable: ${NRIA_AGENT_DIR}"
        exit 1
    fi

    if ${sudoUser} [ ! -r "${NRIA_PLUGIN_DIR}" ]; then
        echo "plugin directory (NRIA_PLUGIN_DIR) not writable: ${NRIA_PLUGIN_DIR}"
        exit 1
    fi

    if ${sudoUser} [ -f "${NRIA_LOG_FILE}" ] && [ ! -w "${NRIA_LOG_FILE}" ]; then
        echo "log file (NRIA_LOG_FILE) not writable: ${NRIA_LOG_FILE}"
        exit 1
    else
        logDir=$(dirname "${NRIA_LOG_FILE}")
        if ${sudoUser} [ ! -w "${logDir}" ]; then
            echo "log directory not writable: ${logDir}"
            exit 1
        fi
    fi
}

# Checking if the installation is rootless or not.
is_root() {
    if [ "${NRIA_MODE}" = "PRIVILEGED" ] || [ "${NRIA_MODE}" = "UNPRIVILEGED" ]; then
        # In non-root mode the user which run the agent should be provided.
        if [ -z "${NRIA_USER}" ]; then
            echo "The environment NRIA_USER is not set and is required for NRIA_MODE: '${NRIA_MODE}'"
            exit 3
        fi
        return 1 # return false
    else
        return 0 # return true
    fi
}

install_upstart_agent() {
    echo "Installing scripts for upstart..."

    # Try stopping the service in case of upgrade.
    initctl stop newrelic-infra || true

    # Prepare Init script.
    service_file="/etc/init/newrelic-infra.conf"
    cp ./etc/init_scripts/upstart/newrelic-infra "${service_file}"

    sed -i "s#/var/run/newrelic-infra#$(dirname ${NRIA_PID_FILE})#g" "${service_file}"
    sed -i "s#/var/db/newrelic-infra/#$(dirname ${NRIA_AGENT_DIR})#g" "${service_file}"

    command="exec "

    if ! is_root; then
        command="exec su -s /bin/sh -c 'exec \"\$0\" \"\$@\"' ${NRIA_USER} -- "
    fi
    sed -i "s#exec /usr/bin/newrelic-infra-service#${command}${agent}#g" "${service_file}"

    install_agent

    # Run the service.
    initctl start newrelic-infra || exit $?
}

install_systemd_agent() {
    echo "Installing scripts for systemd..."

    # Try stopping the service in case of upgrade.
    systemctl stop newrelic-infra || true

    # Prepare Init script.
    service_file="/etc/systemd/system/newrelic-infra.service"
    cp ./etc/init_scripts/systemd/newrelic-infra.service "${service_file}"

    command="ExecStart="
    sed -i "s#ExecStart=/usr/bin/newrelic-infra-service#${command}${agent}#g" "${service_file}"
    sed -i "s#PIDFile=/var/run/newrelic-infra/newrelic-infra.pid#PIDFile=${NRIA_PID_FILE}#g" "${service_file}"

    if ! is_root; then
        grep 'User=' "${service_file}" >/dev/null || sed -i '/\[Service\]/aUser='"${NRIA_USER}" "$service_file"
    fi

    install_agent

    # Set and run the service.
    systemctl daemon-reload || exit $?
    systemctl enable newrelic-infra
    systemctl start newrelic-infra
}

install_sysv_agent() {
    echo "Installing scripts for sysv..."
    service_file=/etc/init.d/newrelic-infra
    cp ./etc/init_scripts/sysv/newrelic-infra "${service_file}"

    # Try stopping the service in case of upgrade.
    ${service_file} stop || true

    if ! is_root; then
        sed -i "s/USER=root/USER=${NRIA_USER}/g" "$service_file"
    fi
    sed -i "s|DAEMON=/usr/bin/\$NAME|DAEMON=${service_binary}\nEXTRA_OPTS=\"${config}\"|g" "$service_file"
    sed -i "s|PIDDIR=/var/run/\$NAME|PIDDIR=$(dirname ${NRIA_PID_FILE})|g" "$service_file"
    sed -i "s|PIDFILE=\$PIDDIR/\$NAME.pid|PIDFILE=${NRIA_PID_FILE}|g" "$service_file"

    # Change command line parameters.
    sed -i "s|--chdir /var/db/newrelic-infra/|--chdir ${NRIA_AGENT_DIR}|g" "$service_file"

    install_agent

    #Set and run the service.
    insserv newrelic-infra || exit $?
    ${service_file} start || exit $?
}

make_dir() {
    if [ ! -d "$1" ] ; then
        mkdir -p "$1"
        if [ ! -z "${NRIA_USER}" ]; then
            chown "${NRIA_USER}" "$1"
        fi
        chmod 0755 "$1"
    fi
}

install_agent() {
    # Copy the binaries and if NRIA_MODE is set to PRIVILEGED set kernel capabilities.
    echo "Installing agent binaries..."
    make_dir "${NRIA_BIN_DIR}"

    cp ./usr/bin/* "${NRIA_BIN_DIR}"

    if [ "$NRIA_MODE" = "PRIVILEGED" ]; then
        # Give the Agent kernel capabilities if setcap command exists.
        set_cap=$(command -v setcap) || set_cap="/sbin/setcap" && [ -f $set_cap ] || set_cap=""
        if [ ! -z "${set_cap}" ]; then
            echo "Adding kernel capabilities..."
            eval "${set_cap} 'CAP_SYS_PTRACE,CAP_DAC_READ_SEARCH=+ep' ${binary}" || exit 1
        fi
    fi

    # Create file structure and copy files.
    echo "Copying agent files..."
    make_dir $(dirname "${NRIA_CONFIG_FILE}")
    touch "${NRIA_CONFIG_FILE}"
    chmod 0644 "${NRIA_CONFIG_FILE}"

    echo "pid_file: ${NRIA_PID_FILE}" > "${NRIA_CONFIG_FILE}"
    echo "plugin_dir: ${NRIA_PLUGIN_DIR}" >> "${NRIA_CONFIG_FILE}"
    echo "agent_dir: ${NRIA_AGENT_DIR}" >> "${NRIA_CONFIG_FILE}"
    echo "log_file: ${NRIA_LOG_FILE}" >> "${NRIA_CONFIG_FILE}"
    echo "license_key: ${NRIA_LICENSE_KEY}" >> "${NRIA_CONFIG_FILE}"

    make_dir $(dirname "${NRIA_PID_FILE}")
    make_dir $(dirname "${NRIA_LOG_FILE}")
    make_dir "${NRIA_PLUGIN_DIR}"

    make_dir "${NRIA_AGENT_DIR}"
    cp -r ./var/db/newrelic-infra/* "${NRIA_AGENT_DIR}"
}

pushd $( dirname $( readlink -f ${BASH_SOURCE[0]} ) )
echo "Installing New Relic infrastructure agent..."
load_config
print_config
install_service
validate
popd
