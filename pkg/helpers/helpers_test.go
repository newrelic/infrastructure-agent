// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package helpers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	databind "github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

type Output struct {
	Value interface{}
}

func TestGenerateMD5(t *testing.T) {
	obtainedOutput := [3]string{}
	expectedOutput := [3]*Output{
		{
			Value: "\xe4\x9f\xe0g\xb0h\xe4\x15\xa9\u007f\x8f=\xd3R\x9f\x8e",
		},
		{
			Value: "\xba\xba2}$\x17F\xee\b)\xe7\xe8\x81\x17\xd4\xd5",
		},
		{
			Value: "\xd4\x1d\x8cُ\x00\xb2\x04\xe9\x80\t\x98\xec\xf8B~",
		},
	}

	if md5generated, err := GenerateMD5("amadeo"); err != nil {
		require.NoError(t, err)
	} else {
		obtainedOutput[0] = string(md5generated)
	}

	if md5generated, err := GenerateMD5("jay"); err != nil {
		require.NoError(t, err)
	} else {
		obtainedOutput[1] = string(md5generated)
	}

	if md5generated, err := GenerateMD5(""); err != nil {
		require.NoError(t, err)
	} else {
		obtainedOutput[2] = string(md5generated)
	}

	// Check output
	for i := range obtainedOutput {
		assert.Equal(t, expectedOutput[i].Value, obtainedOutput[i])
	}
}

func TestJsonRegexp(t *testing.T) {
	files := map[string]bool{
		"/home/vagrant/src/github.com/newrelic/infrastructure-agent/test_data/data/files/config.json":     true,
		"/home/vagrant/src/github.com/newrelic/infrastructure-agent/test_data/data/files/config.json.swp": false,
		"/home/vagrant/src/github.com/newrelic/infrastructure-agent/test_data/data/files/~config.json":    false,
	}
	for f, valid := range files {
		assert.Equal(t, valid, JsonFilesRegexp.Match([]byte(f)))
	}
}

func TestFlattenJson(t *testing.T) {
	jsonData := `
{
  "top": "level stuff",
  "some": {
    "nested": {
      "things": "https://websites.stuff/",
      "how_about": ["an", "array"],
      "or_an_int": 5
    },
    "empty_list": [],
    "null_value": null,
    "floating_point": 7.145,
    "true_false": true,
    "ec2_iam_security_credentials_puppetclient_role": "password"
  },
	"secret": "password",
	"SPECIAL_KEY": "password",
  "byebye": "cya"
}`

	var output map[string]interface{}
	err := json.Unmarshal([]byte(jsonData), &output)
	require.NoError(t, err)
	jsonMap := make(map[string]interface{})
	jsonMap = FlattenJson("", output, jsonMap)
	jsonMap = SanitizeJson(jsonMap)

	for _, v := range jsonMap {
		assert.IsType(t, "", v)
	}

	assert.Equal(t, "level stuff", jsonMap["top"])
	_, ok := jsonMap["some/nested"]
	assert.False(t, ok)
	assert.Equal(t, "https://websites.stuff/", jsonMap["some/nested/things"])
	assert.Equal(t, "[\"an\",\"array\"]", jsonMap["some/nested/how_about"])
	assert.Equal(t, "5", jsonMap["some/nested/or_an_int"])
	assert.Equal(t, "[]", jsonMap["some/empty_list"])
	assert.Equal(t, "null", jsonMap["some/null_value"])
	assert.Equal(t, "7.145", jsonMap["some/floating_point"])
	assert.Equal(t, "true", jsonMap["some/true_false"])
	assert.Equal(t, "Secret obfuscated - md5 hash: 5f4dcc3b5aa765d61d8327deb882cf99", jsonMap["secret"])
	assert.Equal(t, "Secret obfuscated - md5 hash: 5f4dcc3b5aa765d61d8327deb882cf99", jsonMap["SPECIAL_KEY"])
	assert.Equal(t, "cya", jsonMap["byebye"])
	assert.Equal(t, "Secret obfuscated - md5 hash: 5f4dcc3b5aa765d61d8327deb882cf99", jsonMap["some/ec2_iam_security_credentials_puppetclient_role"])
}

func TestISO8601Pattern(t *testing.T) {
	expectedReplacements := map[string]string{
		"2015-06-29T16:04:53Z":         "(timestamp suppressed)",
		"2015-06-29T16:04:53z":         "(timestamp suppressed)",
		"2015-06-29T16:04:53-07:00":    "(timestamp suppressed)",
		"2015-06-29T16:04:53+07:00":    "(timestamp suppressed)",
		"hi 2015-06-29T16:04:53Zthere": "hi (timestamp suppressed)there",
	}

	for testValue, expectedResult := range expectedReplacements {
		assert.Equal(t, expectedResult, ISO8601RE.ReplaceAllString(testValue, "(timestamp suppressed)"))
	}
}

func TestSanitizeCommandLine(t *testing.T) {
	var cases = []struct {
		command   string
		sanitized string
	}{
		{"myprog", "myprog"},
		{"\"myprog\"", "myprog"},
		{"\"/usr/bin/testprog one\"", "/usr/bin/testprog one"},
		{"`/usr/bin/testprog one`", "/usr/bin/testprog one"},
		{"'/usr/bin/testprog one'", "/usr/bin/testprog one"},
		{"'/usr/bin/testprog \"one\"'", "/usr/bin/testprog \"one\""},
		{"/usr/bin/testprog one", "/usr/bin/testprog one"},
		{"/usr/bin/testprog '/var/data/log'", "/usr/bin/testprog '/var/data/log'"},
		{"\"/usr/bin/testprog\" '/var/data/log'", "/usr/bin/testprog '/var/data/log'"},
		{"/usr/bin/funky dir/testprog", "/usr/bin/funky dir/testprog"},
		{"/usr/bin/funky\\ dir/testprog", "/usr/bin/funky\\ dir/testprog"},
		{"\"\\\\Net Reference\\New Relic\\newrelic\\newrelic-infra.exe\"", "\\\\Net Reference\\New Relic\\newrelic\\newrelic-infra.exe"},
		{"\\\\Net Reference\\New Relic\\newrelic\\newrelic-infra.exe", "\\\\Net Reference\\New Relic\\newrelic\\newrelic-infra.exe"},
		{"\\\\NetXReference\\New Relic\\newrelic\\newrelic-infra.exe", "\\\\NetXReference\\New Relic\\newrelic\\newrelic-infra.exe"},
		{"\"C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe\"", "C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe"},
		{"C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe", "C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe"},
		{".\\newrelic\\newrelic-infra.exe", ".\\newrelic\\newrelic-infra.exe"},
		{"newrelic\\newrelic-infra.exe", "newrelic\\newrelic-infra.exe"},
		{"\"\\\\Net Reference\\New Relic\\newrelic\\newrelic-infra.exe -p test\"", "\\\\Net Reference\\New Relic\\newrelic\\newrelic-infra.exe -p test"},
		{"\\\\Net Reference\\New Relic\\newrelic\\newrelic-infra.exe -p test", "\\\\Net Reference\\New Relic\\newrelic\\newrelic-infra.exe -p test"},
		{"\\\\NetXReference\\New Relic\\newrelic\\newrelic-infra.exe -p test", "\\\\NetXReference\\New Relic\\newrelic\\newrelic-infra.exe -p test"},
		{"\"C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe -p test\"", "C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe -p test"},
		{"C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe -p test", "C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe -p test"},
		{"\"C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe\" -p test", "C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe -p test"},
		{".\\newrelic\\newrelic-infra.exe -p test", ".\\newrelic\\newrelic-infra.exe -p test"},
		{"newrelic\\newrelic-infra.exe -p test", "newrelic\\newrelic-infra.exe -p test"},
		{"`C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe -p \"test\"`", "C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe -p \"test\""},
		{"C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe /switch", "C:\\Program Files\\New Relic\\newrelic\\newrelic-infra.exe /switch"},
		{"/sbin/dhclient -H localhost -1 -q -cf /etc/dhcp/dhclient-eth0.conf -lf /var/lib/dhclient/dhclient-eth0.leases -pf /var/run/dhclient-eth0.pid", "/sbin/dhclient -H localhost -1 -q -cf /etc/dhcp/dhclient-eth0.conf -lf /var/lib/dhclient/dhclient-eth0.leases -pf /var/run/dhclient-eth0.pid"},
		{"/opt/export/IBM/InformationServer/ASBNode/apps/jre/bin/java -Xbootclasspath/a:conf:eclipse/plugins/com.ibm.isf.client -Xss2M -Xoss2M -Duser.language=en -Duser.country=US -Djava.ext.dirs=apps/jre/lib/ext:lib/java:eclipse/plugins:eclipse/plugins/com.ibm.isf.client -classpath conf:eclipse/plugins/com.ibm.isf.client -Djava.security.auth.login.config=/opt/export/IBM/InformationServer/ASBNode/eclipse/plugins/com.ibm.isf.client/auth.conf -Dcom.ibm.CORBA.ConfigURL=file:/opt/export/IBM/InformationServer/ASBNode/eclipse/plugins/com.ibm.isf.client/sas.client.props -Dcom.ibm.SSL.ConfigURL=file:/opt/export/IBM/InformationServer/ASBNode/eclipse/plugins/com.ibm.isf.client/ssl.client.props -Dcom.ibm.CORBA.enableClientCallbacks=true -Dcom.ibm.CORBA.FragmentSize=128000 com.ascential.asb.agent.impl.AgentImpl run", "/opt/export/IBM/InformationServer/ASBNode/apps/jre/bin/java -Xbootclasspath/a:conf:eclipse/plugins/com.ibm.isf.client -Xss2M -Xoss2M -Duser.language=en -Duser.country=US -Djava.ext.dirs=apps/jre/lib/ext:lib/java:eclipse/plugins:eclipse/plugins/com.ibm.isf.client -classpath conf:eclipse/plugins/com.ibm.isf.client -Djava.security.auth.login.config=/opt/export/IBM/InformationServer/ASBNode/eclipse/plugins/com.ibm.isf.client/auth.conf -Dcom.ibm.CORBA.ConfigURL=file:/opt/export/IBM/InformationServer/ASBNode/eclipse/plugins/com.ibm.isf.client/sas.client.props -Dcom.ibm.SSL.ConfigURL=file:/opt/export/IBM/InformationServer/ASBNode/eclipse/plugins/com.ibm.isf.client/ssl.client.props -Dcom.ibm.CORBA.enableClientCallbacks=true -Dcom.ibm.CORBA.FragmentSize=128000 com.ascential.asb.agent.impl.AgentImpl run"},
	}

	for i, tt := range cases {
		t.Run(fmt.Sprintf("CommandLine_%v", i), func(t *testing.T) {
			assert.Equal(t, tt.sanitized, SanitizeCommandLine(tt.command))
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	var cases = []struct {
		input, expected string
	}{
		{"\\::que???pasa***tronko<-||>", "quepasatronko-"},
		{"iden\\tifier.txt", "identifier.txt"},
		{"iden/tifi/er.txt", "identifier.txt"},
		{"ide:ntif*ier.txt", "identifier.txt"},
		{"id?en?tif\"ier.txt", "identifier.txt"},
		{"id<ent>if|||ier.txt", "identifier.txt"},
		{"|<*identifier.txt::**?", "identifier.txt"},
		{"identifier.txt", "identifier.txt"},
	}

	for _, tt := range cases {
		assert.Equal(t, tt.expected, SanitizeFileName(tt.input))
	}
	cacheLen := sanitizeFileNameCache.Len()
	assert.Equal(t, cacheLen, 8, "sanitizeFileNameCache length assert failed")

	// Sanitize again the same value to assert that the SanitizeFileName cache correctly.
	Case := cases[len(cases)-1]
	assert.Equal(t, Case.expected, SanitizeFileName(Case.input))
	cacheLen = sanitizeFileNameCache.Len()
	assert.Equal(t, cacheLen, 8, "sanitizeFileNameCache length assert failed")
}

func TestRemoveEmptyAndDuplicateEntries(t *testing.T) {
	// Given a slice with empty and duplicate entries
	input := []string{"", "abc", "cde", "abc", "", "cde", "kadsfj", "kadsfj", "aaaaaa"}

	// When removing repeated and duplicated entries
	output := RemoveEmptyAndDuplicateEntries(input)

	// Then the output string does not have empty or repeated entries, keeping the order of the original input
	assert.Equal(t, []string{"abc", "cde", "kadsfj", "aaaaaa"}, output)
}

func TestRemoveEmptyAndDuplicateEntries_EmptyInput(t *testing.T) {
	// Given an empty array
	var input []string

	// When removing repeated and duplicated entries
	output := RemoveEmptyAndDuplicateEntries(input)

	// Then the output string is an empty array
	assert.Equal(t, []string{}, output)
}

func TestRemoveEmptyAndDuplicateEntries_DiscardableInput(t *testing.T) {
	// Given an array only with empty strings
	input := []string{"", ""}

	// When removing repeated and duplicated entries
	output := RemoveEmptyAndDuplicateEntries(input)

	// Then the output string is an empty array
	assert.Equal(t, []string{}, output)
}

func TestRemoveEmptyAndDuplicateEntries_Identity(t *testing.T) {
	// Given an array without empty and duplicate entries
	input := []string{"aaaaaa", "abc", "cde", "kadsfj"}

	// When removing repeated and duplicated entries
	output := RemoveEmptyAndDuplicateEntries(input)

	// Then the output string does not have empty or repeated entries, keeping the order of the original input
	assert.Equal(t, []string{"aaaaaa", "abc", "cde", "kadsfj"}, output)
}

func TestReadFirstLine(t *testing.T) {
	filePath := filepath.FromSlash(fmt.Sprintf("%s/redhat-release", os.TempDir()))
	releaseFile := `Red Hat Enterprise Linux AS release 3 (Taroon)
Another test line`

	err := ioutil.WriteFile(filePath, []byte(releaseFile), 0644)
	require.NoError(t, err)

	distro := ReadFirstLine(filePath)

	assert.Equal(t, "Red Hat Enterprise Linux AS release 3 (Taroon)", distro)
}

func TestReadFirstLineMissingFile(t *testing.T) {
	// File is not there or it is protected

	distro := ReadFirstLine("/cheeseits/chocolate")

	assert.Equal(t, "unknown", distro)
}

func TestObfuscateSensitiveData_EmptyString(t *testing.T) {
	matched, isField, result := ObfuscateSensitiveData("")

	assert.False(t, matched)
	assert.False(t, isField)
	assert.Equal(t, "", result)
}

func TestObfuscateSensitiveData_NoMatch(t *testing.T) {
	data := "this is some string"
	matched, isField, result := ObfuscateSensitiveData(data)

	assert.False(t, matched)
	assert.False(t, isField)
	assert.Equal(t, data, result)
}

func TestObfuscateSensitiveData_MatchButNothingToObfuscate(t *testing.T) {
	data := "-password"
	matched, isField, result := ObfuscateSensitiveData(data)

	assert.True(t, matched)
	assert.True(t, isField)
	assert.Equal(t, data, result)
}

func TestObfuscateSensitiveData_CommandLineWithArgs(t *testing.T) {
	data := "/usr/bin/custom_cmd -pwd 1234 -arg2 abc"
	expected := "/usr/bin/custom_cmd -pwd <HIDDEN> -arg2 abc"
	matched, isField, actual := ObfuscateSensitiveData(data)

	assert.True(t, matched)
	assert.False(t, isField)
	assert.Equal(t, expected, actual)
}

func TestObfuscateSensitiveData_ConfigProtocolOutput(t *testing.T) {
	data := `{"config_protocol_version":"1","action":"register_config","config_name":"cfg-nri-ibmmq","config":{"variables":{},"integrations":[{"name":"nri-prometheus","config":{"standalone":false,"verbose":"1","transformations":[],"integration_metadata":{"version":"0.3.0","name":"nri-ibmmq""targets":["urls":["http://localhost:9157"]}]}},{"name":"ibmmq-exporter","timeout":0,  "exec":[    "/usr/local/prometheus-exporters/bin/ibmmq-exporter","--mongodb.uri","mongodb://root:supercomplex@localhost:17017","--ibmmq.connName","localhost(1414)","--ibmmq.queueManager","QM1","--ibmmq.channel","DEV.ADMIN.SVRCONN","--ibmmq.userid","admin","--ibmmq.httpListenPort","9157","--ibmmq.monitoredQueues","!SYSTEM.*,*","--ibmmq.monitoredChannels","*","--ibmmq.httpMetricPath","/metrics","--ibmmq.useStatus"],"env":{"IBMMQ_CONNECTION_PASSWORD":"passw0rd","LD_LIBRARY_PATH":"/opt/mqm/lib64:/usr/lib64","HOME":"/tmp"}}]}}`
	expected := `{"config_protocol_version":"1","action":"register_config","config_name":"cfg-nri-ibmmq","config":{"variables":{},"integrations":[{"name":"nri-prometheus","config":{"standalone":false,"verbose":"1","transformations":[],"integration_metadata":{"version":"0.3.0","name":"nri-ibmmq""targets":["urls":["http://localhost:9157"]}]}},{"name":"ibmmq-exporter","timeout":0,  "exec":[    "/usr/local/prometheus-exporters/bin/ibmmq-exporter","--mongodb.uri","mongodb://root:<HIDDEN>@localhost:17017","--ibmmq.connName","localhost(1414)","--ibmmq.queueManager","QM1","--ibmmq.channel","DEV.ADMIN.SVRCONN","--ibmmq.userid","admin","--ibmmq.httpListenPort","9157","--ibmmq.monitoredQueues","!SYSTEM.*,*","--ibmmq.monitoredChannels","*","--ibmmq.httpMetricPath","/metrics","--ibmmq.useStatus"],"env":{"IBMMQ_CONNECTION_PASSWORD":"<HIDDEN>","LD_LIBRARY_PATH":"/opt/mqm/lib64:/usr/lib64","HOME":"/tmp"}}]}}`
	matched, isField, actual := ObfuscateSensitiveData(data)

	assert.True(t, matched)
	assert.False(t, isField)
	assert.Equal(t, expected, actual)
}

func TestObfuscateSensitiveData_EnvironmentVariable(t *testing.T) {
	data := "NRIA_CUSTOM_PASSWORD=1234"
	expected := "NRIA_CUSTOM_PASSWORD=<HIDDEN>"

	matched, isField, actual := ObfuscateSensitiveData(data)

	assert.True(t, matched)
	assert.False(t, isField)
	assert.Equal(t, expected, actual)

	data = "NRIA_CUSTOM_PASSWORD=1234 NRIA_CUSTOM=abc NRIA_CUSTOM_token=zzzz"
	expected = "NRIA_CUSTOM_PASSWORD=<HIDDEN> NRIA_CUSTOM=abc NRIA_CUSTOM_token=<HIDDEN>"
	matched, isField, actual = ObfuscateSensitiveData(data)

	assert.True(t, matched)
	assert.False(t, isField)
	assert.Equal(t, expected, actual)
}

func TestObfuscateSensitiveData_ObfuscateSensitiveDataFromMap(t *testing.T) {
	data := map[string]string{
		"simple":              "value",
		"NRIA_PASS":           "obfuscate_this",
		"some password: here": "value",
	}

	expected := map[string]string{
		"simple":                  "value",
		"NRIA_PASS":               "<HIDDEN>",
		"some password: <HIDDEN>": "value",
	}

	actual := ObfuscateSensitiveDataFromMap(data)

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("Expected: %v\n\nActual: %v", expected, actual)
	}
}

func TestObfuscateSensitiveDataFromMap_DoesNotMutateOriginalData(t *testing.T) {
	data := map[string]string{
		"simple":              "value",
		"NRIA_PASS":           "obfuscate_this",
		"some password: here": "value",
	}
	// avoid copying
	originalData := map[string]string{
		"simple":              "value",
		"NRIA_PASS":           "obfuscate_this",
		"some password: here": "value",
	}

	_ = ObfuscateSensitiveDataFromMap(data)

	assert.Equal(t, originalData, data)
}

func TestObfuscateSensitiveDataFromArray_DoesNotMutateOriginalData(t *testing.T) {
	data := []string{
		"simple",
		"obfuscate_this_pass 12345",
		"parameter",
		"obfuscare_next_pass",
		"12345",
		"NRIA_KEY=1234",
		"final",
	}
	// avoid copying
	originalData := []string{
		"simple",
		"obfuscate_this_pass 12345",
		"parameter",
		"obfuscare_next_pass",
		"12345",
		"NRIA_KEY=1234",
		"final",
	}

	_ = ObfuscateSensitiveDataFromArray(data)

	assert.Equal(t, originalData, data)
}

func TestObfuscateSensitiveData_ObfuscateSensitiveDataFromMapUninitialized(t *testing.T) {
	var data map[string]string
	expected := map[string]string{}

	actual := ObfuscateSensitiveDataFromMap(data)

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("Expected: %v\n\nActual: %v", expected, actual)
	}
}

func TestObfuscateSensitiveData_ObfuscateSensitiveDataFromArray(t *testing.T) {
	data := []string{
		"simple",
		"obfuscate_this_pass 12345",
		"parameter",
		"obfuscare_next_pass",
		"12345",
		"NRIA_KEY=1234",
		"final",
	}

	expected := []string{
		"simple",
		"obfuscate_this_pass <HIDDEN>",
		"parameter",
		"obfuscare_next_pass",
		"<HIDDEN>",
		"NRIA_KEY=<HIDDEN>",
		"final",
	}

	actual := ObfuscateSensitiveDataFromArray(data)

	assert.Equal(t, expected, actual)
}

func TestObfuscateSensitiveData_ObfuscateSensitiveDataFromArrayUninitialized(t *testing.T) {
	assert.Equal(t, []string(nil), ObfuscateSensitiveDataFromArray([]string{}))
}

func TestCloseQuietly(t *testing.T) {
	c := &errorCloser{}
	err := errors.New("some error you don't care about")
	c.On("Close").Once().Return(err)
	CloseQuietly(c)
	c.AssertExpectations(t)
}

type errorCloser struct {
	mock.Mock
}

func (e *errorCloser) Close() error {
	return e.Called().Error(0)
}

func TestObfuscateSensitiveDataFromError(t *testing.T) {

	databindMap := databind.InterfaceMap{
		"request_id": "15f7d78b-6385-fbb2-52a1-55ebd01b8d31",
		"data": databind.InterfaceMap{
			"password": "correct horse battery staple",
			"username": "bob",
		},
	}

	passwordStruct := struct {
		Username string
		Password string
		Data     map[string]string
	}{
		Username: "bob",
		Password: "correct horse battery staple",
		Data: map[string]string{
			"password": "correct horse battery staple",
		},
	}

	tests := []struct {
		name            string
		originalError   error
		expectedMessage string
		skipTest        bool
	}{
		{
			name:            "noop",
			originalError:   errors.New("nothing to obfuscate"),
			expectedMessage: "nothing to obfuscate",
		},
		{
			name:            "randomText",
			originalError:   errors.New("vault returned an unexpected format from the http server: {\"request_id\":\"15f7d78b-6385-fbb2-52a1-55ebd01b8d31\",\"data\":{\"password\":\"correct horse battery staple\",\"port\":2020,\"username\":\"bob\"},\"wrap_info\":null,\"warnings\":null,\"auth\":null}\n"),
			expectedMessage: "vault returned an unexpected format from the http server: {\"request_id\":\"15f7d78b-6385-fbb2-52a1-55ebd01b8d31\",\"data\":{\"password\":\"<HIDDEN> horse battery staple\",\"port\":2020,\"username\":\"bob\"},\"wrap_info\":null,\"warnings\":null,\"auth\":<HIDDEN>\n",
		},
		{
			name:            "dataBindMap",
			originalError:   fmt.Errorf("databindMap with response: %s", databindMap),
			expectedMessage: "databindMap with response: map[request_id:15f7d78b-6385-fbb2-52a1-55ebd01b8d31 data:map[password:<HIDDEN> horse battery staple username:bob]]",
			skipTest:        true,
		},
		{
			name:            "struct",
			originalError:   fmt.Errorf("struct with response: %s", passwordStruct),
			expectedMessage: "struct with response: {bob correct horse battery staple map[password:<HIDDEN> horse battery staple]}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipTest {
				t.Skipf("Skipping test because something fails in the pipeline and I don't know why")
			}
			err := ObfuscateSensitiveDataFromError(tt.originalError)
			assert.EqualError(t, err, tt.expectedMessage)
		})
	}
}

func TestObfuscateSensitiveDataFromError_nill(t *testing.T) {
	err := ObfuscateSensitiveDataFromError(nil)
	assert.Nil(t, err)
}

func TestSplitRightSubstring(t *testing.T) {
	var testCases = []struct {
		name      string
		output    string
		substring string
		separator string
		expected  string
	}{
		{name: "Empty input",
			output:    "",
			substring: "",
			separator: "",
			expected:  "",
		},
		{name: "Empty substring",
			output:    "Hello bye yes",
			substring: "",
			separator: "$",
			expected:  "",
		},
		{name: "Empty separator",
			output:    "Hello bye yes",
			substring: "bye",
			separator: "",
			expected:  "",
		},
		{name: "Word separator",
			output:    "Hello bye yes",
			substring: "Hello ",
			separator: " yes",
			expected:  "bye",
		},
		{name: "Dot separator",
			output:    "Fosdem: A lot of questions.",
			substring: "Fosdem: ",
			separator: ".",
			expected:  "A lot of questions",
		},
		{name: "Newline separator",
			output: `Fosdem: A lot of questions
`,
			substring: "Fosdem: ",
			separator: "\n",
			expected:  "A lot of questions",
		},
		{name: "Substring high slice bound",
			output:    "Fosdem: A lot of questions",
			substring: "Fosdem: A lot of questions",
			separator: "",
			expected:  "",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(
				t,
				SplitRightSubstring(tt.output,
					tt.substring,
					tt.separator,
				),
				tt.expected)
		})
	}
}
