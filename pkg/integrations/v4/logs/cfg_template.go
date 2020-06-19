// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package logs

var fbConfigFormat = `{{- range .Inputs }}
[INPUT]
    Name {{ .Name }}
    {{- if .Path }}
    Path {{ .Path }}
    {{- end }}
    {{- if .BufferChunkSize }}
    Buffer_Chunk_Size {{ .BufferChunkSize }}
    {{- end }}
    {{- if .BufferMaxSize }}
    Buffer_Max_Size {{ .BufferMaxSize }}
    {{- end }}
    {{- if .SkipLongLines }}
    Skip_Long_Lines {{ .SkipLongLines }}
    {{- end }}
    {{- if .Tag }}
    Tag  {{ .Tag }}
    {{- end }}
    {{- if .DB }}
    DB   {{ .DB }}
    {{- end }}
    {{- if .Systemd_Filter }}
    Systemd_Filter {{ .Systemd_Filter }}
    {{- end }}
    {{- if .Channels }}
    Channels {{ .Channels }}
    {{- end }}
    {{- if .SyslogMode }}
    Mode {{ .SyslogMode }}
    {{- end }}
    {{- if .SyslogListen }}
    Listen {{ .SyslogListen }}
    {{- end }}
    {{- if .SyslogPort }}
    Port {{ .SyslogPort }}
    {{- end }}
    {{- if .SyslogParser }}
    Parser {{ .SyslogParser }}
    {{- end }}
    {{- if .SyslogUnixPath }}
    Path {{ .SyslogUnixPath }}
    {{- end }}
    {{- if .SyslogUnixPermissions }}
    Unix_Perm {{ .SyslogUnixPermissions }}
    {{- end }}
    {{- if .TcpListen }}
    Listen {{ .TcpListen }}
    {{- end }}
    {{- if .TcpPort }}
    Port {{ .TcpPort }}
    {{- end }}
    {{- if .TcpFormat }}
    Format {{ .TcpFormat }}
    {{- end }}
    {{- if .TcpSeparator }}
    Separator {{ .TcpSeparator }}
    {{- end }}
    {{- if .TcpBufferSize }}
    Buffer_Size {{ .TcpBufferSize }}
    {{- end }}
{{ end -}}

{{- range .Parsers }}
[FILTER]
    {{- if .Name }}
    Name  {{ .Name }}
    {{- end }}
    {{- if .Match }}
    Match {{ .Match }}
    {{- end }}
    {{- if .Regex }}
    Regex {{ .Regex }}
    {{- end }}
    {{- if .Records }}
        {{- range $key, $value := .Records }}
    Record {{ $key }} {{ $value }}
        {{- end }}
    {{- end }}
{{ end -}}

{{- if .Output }}
[OUTPUT]
    Name                {{ .Output.Name }}
    Match               {{ .Output.Match }}
    {{- if .Output.LicenseKey }}
    licenseKey          {{ .Output.LicenseKey }}
    {{- end }}
    {{- if .Output.Endpoint }}
    endpoint            {{ .Output.Endpoint }}
    {{- end }}
    {{- if .Output.Proxy }}
    proxy               {{ .Output.Proxy }}
    {{- end }}
	{{- if .Output.IgnoreSystemProxy }}
    ignoreSystemProxy   true
    {{- end }}
	{{- if .Output.CABundleFile }}
    caBundleFile        {{ .Output.CABundleFile }}
    {{- end }}
    {{- if .Output.CABundleDir }}
    caBundleDir         {{ .Output.CABundleDir }}
    {{- end }}
    {{- if not .Output.ValidateCerts }}
    validateProxyCerts  false
    {{- end }}
{{ end -}}

{{- if .ExternalCfg.CfgFilePath }}
@INCLUDE {{ .ExternalCfg.CfgFilePath }}
{{ end -}}`
