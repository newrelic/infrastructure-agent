// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const yamlIndent = 2

// FormatYAML will return the FBCfg in the Fluent Bit YAML config file format.
// It produces the same pipeline as Format (classic), key by key.
//
// The "includes" section is written after "pipeline" on purpose: Fluent Bit merges included
// files in document order, so this keeps external filters running after the agent filters,
// as they do with the classic "@INCLUDE" at the end of the file.
func (c FBCfg) FormatYAML() (result string, externalCfg FBCfgExternal, err error) {
	pipeline := newYAMLMap()
	if len(c.Inputs) > 0 {
		inputs := newYAMLSeq()
		for _, in := range c.Inputs {
			inputs.Content = append(inputs.Content, yamlInput(in))
		}
		pipeline.add("inputs", inputs)
	}

	if len(c.Filters) > 0 {
		filters := newYAMLSeq()
		for _, f := range c.Filters {
			filters.Content = append(filters.Content, yamlFilter(f))
		}
		pipeline.add("filters", filters)
	}

	if (c.Output != FBCfgOutput{}) {
		outputs := newYAMLSeq()
		outputs.Content = append(outputs.Content, yamlOutput(c.Output))
		pipeline.add("outputs", outputs)
	}

	root := newYAMLMap()
	root.add("pipeline", pipeline.node)

	if c.ExternalCfg.CfgFilePath != "" {
		includes := newYAMLSeq()
		includes.Content = append(includes.Content, yamlStr(c.ExternalCfg.CfgFilePath))
		root.add("includes", includes)
	}

	buf := new(bytes.Buffer)
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(yamlIndent)
	if err = enc.Encode(root.node); err != nil {
		return "", FBCfgExternal{}, errors.Wrap(err, "cannot write log-forwarder yaml config")
	}
	if err = enc.Close(); err != nil {
		return "", FBCfgExternal{}, errors.Wrap(err, "cannot write log-forwarder yaml config")
	}

	return buf.String(), c.ExternalCfg, nil
}

// yamlInput mirrors the [INPUT] block of fbConfigFormat, keeping the same keys and order.
//
//nolint:cyclop
func yamlInput(in FBCfgInput) *yaml.Node {
	m := newYAMLMap()
	m.addStr("name", in.Name)
	m.addStr("path", in.Path)
	m.addStr("buffer_chunk_size", in.BufferChunkSize)
	m.addStr("buffer_max_size", in.BufferMaxSize)
	m.addStr("mem_buf_limit", in.MemBufferLimit)
	m.addStr("skip_long_lines", in.SkipLongLines)
	m.addStr("multiline.parser", in.MultilineParser)
	m.addStr("unicode.encoding", in.UnicodeEncoding)
	m.addStr("path_key", in.PathKey)
	m.addStr("tag", in.Tag)
	m.addStr("db", in.DB)
	m.addStr("systemd_filter", in.Systemd_Filter)
	m.addStr("channels", in.Channels)
	m.addStr("mode", in.SyslogMode)
	m.addStr("listen", in.SyslogListen)
	m.addInt("port", in.SyslogPort)
	m.addStr("parser", in.SyslogParser)
	m.addStr("path", in.SyslogUnixPath)
	m.addStr("unix_perm", in.SyslogUnixPermissions)
	m.addStr("listen", in.TcpListen)
	m.addInt("port", in.TcpPort)
	m.addStr("format", in.TcpFormat)
	m.addStr("separator", in.TcpSeparator)
	m.addInt("buffer_size", in.TcpBufferSize)
	m.addStr("use_ansi", in.UseANSI)

	return m.node
}

// yamlFilter mirrors the [FILTER] block of fbConfigFormat. Repeated classic keys
// ("Record", "Rename") become YAML lists with the same "key value" strings.
func yamlFilter(f FBCfgFilter) *yaml.Node {
	m := newYAMLMap()
	m.addStr("name", f.Name)
	m.addStr("match", f.Match)
	m.addStr("regex", f.Regex)

	if len(f.Records) > 0 {
		records := newYAMLSeq()
		for _, key := range sortedKeys(f.Records) {
			records.Content = append(records.Content, yamlStr(fmt.Sprintf(`"%s" "%s"`, key, f.Records[key])))
		}
		m.add("record", records)
	}

	if len(f.Modifiers) > 0 {
		renames := newYAMLSeq()
		for _, key := range sortedKeys(f.Modifiers) {
			renames.Content = append(renames.Content, yamlStr(fmt.Sprintf("%s %s", key, f.Modifiers[key])))
		}
		m.add("rename", renames)
	}

	m.addStr("script", f.Script)
	m.addStr("call", f.Call)

	return m.node
}

// yamlOutput mirrors the [OUTPUT] block of fbConfigFormat.
//
// The New Relic output plugin keys must be all lowercase: Fluent Bit rewrites camelCase keys of YAML
// files to snake_case before the plugin reads them (licenseKey -> license_key), and the plugin lookup
// ignores case but not underscores, so camelCase keys would be silently lost. Lowercase keys are left
// untouched and still match the plugin lookup.
func yamlOutput(out FBCfgOutput) *yaml.Node {
	m := newYAMLMap()
	m.addStr("name", out.Name)
	m.addStr("match", out.Match)
	if out.LicenseKey != "" {
		m.addStr("licensekey", "${NR_LICENSE_KEY_ENV_VAR}")
	}
	m.addStr("endpoint", out.Endpoint)
	m.addStr("proxy", out.Proxy)
	if out.IgnoreSystemProxy {
		m.addStr("ignoresystemproxy", "true")
	}
	m.addStr("cabundlefile", out.CABundleFile)
	m.addStr("cabundledir", out.CABundleDir)
	if !out.ValidateCerts {
		m.addStr("validateproxycerts", "false")
	}
	m.addStr("retry_limit", out.Retry_Limit)
	m.addStr("httpclienttimeout", out.HTTPClientTimeout)
	if out.SendMetrics {
		m.addStr("sendmetrics", "true")
	}

	return m.node
}

// yamlMap is an ordered YAML mapping that skips empty values, like the classic template does.
type yamlMap struct {
	node *yaml.Node
}

func newYAMLMap() yamlMap {
	return yamlMap{node: &yaml.Node{Kind: yaml.MappingNode}}
}

func newYAMLSeq() *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode}
}

func yamlStr(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func (m yamlMap) add(key string, value *yaml.Node) {
	m.node.Content = append(m.node.Content, yamlStr(key), value)
}

func (m yamlMap) addStr(key, value string) {
	if value != "" {
		m.add(key, yamlStr(value))
	}
}

func (m yamlMap) addInt(key string, value int) {
	if value != 0 {
		m.add(key, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)})
	}
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}
