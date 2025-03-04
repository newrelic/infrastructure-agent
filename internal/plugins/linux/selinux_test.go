// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
//go:build linux
// +build linux

//nolint:revive
package linux

import (
	"testing"

	. "gopkg.in/check.v1"
)

// Register test suite.
func TestSELinux(t *testing.T) {
	t.Parallel()
}

type SELinuxSuite struct{}

var _ = Suite(&SELinuxSuite{})

var sampleOutput = `SELinux status:                 enabled
SELinuxfs mount:                /selinux
Current mode:                   permissive
Mode from config file:          permissive
Policy version:                 24
Policy from config file:        targeted

Policy booleans:
abrt_anon_write                             off
abrt_handle_event                           off
allow_console_login                         on
allow_cvs_read_shadow                       off
allow_httpd_mod_auth_ntlm_winbind           off
allow_user_mysql_connect                    off
allow_user_postgresql_connect               off
daemons_enable_cluster_mode                 on
dhcpc_exec_iptables                         off
gluster_export_all_rw                       on
httpd_use_gpg                               off
nsplugin_can_network                        on
openshift_use_nfs                           off
rsync_use_nfs                               off
samba_create_home_dirs                      off
samba_domain_controller                     off
samba_enable_home_dirs                      off
samba_export_all_ro                         off
samba_export_all_rw                         off
tor_bind_all_unreserved_ports               off
unconfined_login                            on
xserver_object_manager                      off
zabbix_can_network                          off
`

func (ss *SELinuxSuite) TestParseSELinuxConfig(c *C) {
	plugin := SELinuxPlugin{}

	result, policies, err := plugin.parseSestatusOutput(sampleOutput)
	if err != nil {
		c.Fatal(err)
	}

	resultMap := make(map[string]string)
	for _, entity := range result {
		resultMap[entity.SortKey()] = entity.(SELinuxConfigValue).Value
	}

	policyMap := make(map[string]string)
	for _, entity := range policies {
		policyMap[entity.SortKey()] = entity.(SELinuxConfigValue).Value
	}

	c.Check(resultMap["FSMount"], Equals, "/selinux")
	c.Check(resultMap["PolicyVersion"], Equals, "24")
	c.Check(policyMap["allow_console_login"], Equals, "on")
	c.Check(policyMap["samba_domain_controller"], Equals, "off")
}

var sampleOutputDisabled = `SELinux status:                 disabled
`

func (ss *SELinuxSuite) TestSELinuxDisabledCheck(c *C) {
	plugin := SELinuxPlugin{}

	_, _, err := plugin.parseSestatusOutput(sampleOutputDisabled)
	c.Assert(err, Not(IsNil))
	c.Check(err, Equals, ErrSELinuxDisabled)
}

var sampleSemoduleOutput = `abrt	1.2.0
accountsd	1.0.0
ada	1.4.0
afs	1.5.3
aiccu	1.0.0
aide	1.5.0
amanda	1.12.0
amtu	1.2.0
antivirus	1.0.0
apache	2.1.2
apcupsd	1.6.1
arpwatch	1.8.1
asterisk	1.7.1
audioentropy	1.6.0
automount	1.12.1
avahi	1.11.2
awstats	1.2.0
`

func (ss *SELinuxSuite) TestParseSEModules(c *C) {
	plugin := SELinuxPlugin{}

	result := plugin.parseSemoduleOutput(sampleSemoduleOutput)

	resultMap := make(map[string]string)
	for _, entity := range result {
		resultMap[entity.SortKey()] = entity.(SELinuxPolicyModule).Version
	}

	c.Check(resultMap["aiccu"], Equals, "1.0.0")
	c.Check(resultMap["audioentropy"], Equals, "1.6.0")
	c.Check(len(resultMap), Equals, 17)
}

func (ss *SELinuxSuite) TestParseSEModulesEmptyVersionCheck(chk *C) {
	//exhaustruct:ignore
	plugin := SELinuxPlugin{}
	var sampleSemoduleOutputWithoutVersions = `abrt
	accountsd
	ada
	afs
	aiccu
	aide
	amanda
	amtu
	antivirus
	apache
	apcupsd
	arpwatch
	asterisk
	audioentropy
	automount
	avahi
	awstats
	`
	result := plugin.parseSemoduleOutput(sampleSemoduleOutputWithoutVersions)

	resultMap := make(map[string]string)
	for _, entity := range result {
		key := entity.SortKey()
		seLinuxPolicyModule, ok := entity.(SELinuxPolicyModule)

		if !ok {
			chk.Fatal("error occurred!!")
		}
		resultMap[key] = seLinuxPolicyModule.Version
	}

	chk.Check(resultMap["abrt"], Equals, "")
	chk.Check(resultMap["accountsd"], Equals, "")
	chk.Check(len(resultMap), Equals, 17)
}
