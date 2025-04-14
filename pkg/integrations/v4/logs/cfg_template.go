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
	{{- if .MemBufferLimit }}
    Mem_Buf_Limit {{ .MemBufferLimit }}
    {{- end }}
    {{- if .SkipLongLines }}
    Skip_Long_Lines {{ .SkipLongLines }}
    {{- end }}
    {{- if .MultilineParser }}
    Multiline.Parser {{ .MultilineParser }}
    {{- end }}
    {{- if .PathKey }}
    Path_Key {{ .PathKey }}
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
 	{{- if .UseANSI }}
    Use_ANSI {{ .UseANSI }}
    {{- end }}
    {{- if .Alias }}
    Alias {{ .Alias }}
    {{- end }}
    {{- if .Port }}
    Port {{ .Port }}
    {{- end }}
    {{- if .Host }}
    Host {{ .Host }}
    {{- end }}
    {{- if .MetricsPath }}
    Metrics_Path {{ .MetricsPath }}
    {{- end }}
    {{- if .ScrapeInterval }}
    Scrape_Interval {{ .ScrapeInterval }}
    {{- end }}
{{ end -}}

{{- range .Filters }}
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
    Record "{{ $key }}" "{{ $value }}"
        {{- end }}
    {{- end }}
    {{- if .Modifiers }}
        {{- range $key, $value := .Modifiers }}
    Rename {{ $key }} {{ $value }}
        {{- end }}
    {{- end }}
    {{- if .Script }}
    script {{ .Script }}
    {{- end }}
    {{- if .Call }}
    call {{ .Call }}
    {{- end }}
{{ end -}}

{{- range .Service }}
[SERVICE]
    {{- if .Flush }}
    Flush         {{ .Flush }}
    {{- end }}
    {{- if .LogLevel }}
    Log_Level     {{ .LogLevel }}
    {{- end }}
    {{- if .Daemon }}
    Daemon        {{ .Daemon }}
    {{- end }}
    {{- if .ParsersFile }}
    Parsers_File  {{ .ParsersFile }}
    {{- end }}
    {{- if .HTTPServer }}
    HTTP_Server   {{ .HTTPServer }}
    {{- end }}
    {{- if .HTTPListen }}
    HTTP_Listen   {{ .HTTPListen }}
    {{- end }}
    {{- if .HTTPPort }}
    HTTP_Port     {{ .HTTPPort }}
    {{- end }}
{{ end -}}

 
{{- range .Output }}
[OUTPUT]
    Name                {{ .Name }}
    Match               {{ .Match }}
    {{- if .LicenseKey }}
    licenseKey          ${NR_LICENSE_KEY_ENV_VAR}
    {{- end }}
    {{- if .Endpoint }}
    endpoint            {{ .Endpoint }}
    {{- end }}
    {{- if .Proxy }}
    proxy               {{ .Proxy }}
    {{- end }}
	{{- if .IgnoreSystemProxy }}
    ignoreSystemProxy   true
    {{- end }}
	{{- if .CABundleFile }}
    caBundleFile        {{ .CABundleFile }}
    {{- end }}
    {{- if .CABundleDir }}
    caBundleDir         {{ .CABundleDir }}
    {{- end }}
    {{- if not .ValidateCerts }}
    {{- if eq .Name "newrelic" }}
    validateProxyCerts  false
    {{- end }}
    {{- end }}
    {{- if .Retry_Limit}}
    Retry_Limit         {{ .Retry_Limit }}
    {{- end }}
    {{- if .SendMetrics}}
    sendMetrics         {{ .SendMetrics}}
    {{- end}}
    {{- if .Alias }}
    Alias               {{ .Alias }}
    {{- end }}
    {{- if .Host }}
    Host                {{ .Host }}
    {{- end }}
    {{- if .Port }}
    Port                {{ .Port }}
    {{- end }}
    {{- if .URI }}
    Uri                 {{ .URI }}
    {{- end }}
    {{- if .Header }}
    Header              {{ .Header }}
    {{- end }}
    {{- if .TLS }}
    Tls                 {{ .TLS }}
    {{- end }}
    {{- if .TLSVerify }}
    Tls.verify          {{ .TLSVerify }}
    {{- end }}
    {{- if .AddLabel }}
        {{- range $key, $value := .AddLabel }}
    add_label           {{ $key }} {{ $value }}
        {{- end }}
    {{- end }}
{{ end -}}

{{- if .ExternalCfg.CfgFilePath }}
@INCLUDE {{ .ExternalCfg.CfgFilePath }}
{{ end -}}`

var fbLuaScriptFormat = `function {{ .FnName }}(tag, timestamp, record)
    eventId = record["EventID"]
    -- Discard log records matching any of these conditions
    if {{ .ExcludedEventIds }} then
        return -1, 0, 0
    end
    -- Include log records matching any of these conditions
    if {{ .IncludedEventIds }} then
        return 0, 0, 0
    end
    -- If there is not any matching conditions discard everything
    return -1, 0, 0
 end`
