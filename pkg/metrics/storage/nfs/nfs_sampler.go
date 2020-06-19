// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package nfs

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/config"
	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/sample"
	"github.com/prometheus/procfs"
)

var sslog = log.WithComponent("NFSSampler")

type Sampler struct {
	context     agent.AgentContext
	lastRun     time.Time
	lastSamples map[string]statsCache
	sampleRate  time.Duration
	detailed    bool
}

type statsCache struct {
	last    *procfs.MountStatsNFS
	lastRun time.Time
}

type Sample struct {
	sample.BaseEvent

	// Total number of available bytes on disk
	DiskTotalBytes *uint64 `json:"diskTotalBytes,omitempty"`
	// Total number of bytes used
	DiskUsedBytes *uint64 `json:"diskUsedBytes,omitempty"`
	// Percentage of bytes used
	DiskUsedPercent *float64 `json:"diskUsedPercent,omitempty"`
	// Total number of available bytes left on disk
	DiskFreeBytes *uint64 `json:"diskFreeBytes,omitempty"`
	// Percent of free space available on disk
	DiskFreePercent *float64 `json:"diskFreePercent,omitempty"`
	// Total number of bytes read
	TotalReadBytes *uint64 `json:"totalReadBytes,omitempty"`
	// Total number of bytes written
	TotalWriteBytes *uint64 `json:"totalWriteBytes,omitempty"`
	// Number of bytes read per second
	ReadBytesPerSec *float64 `json:"readBytesPerSecond,omitempty"`
	// Number of bytes written per second
	WriteBytesPerSec *float64 `json:"writeBytesPerSecond,omitempty"`
	// Number of read operations per second
	ReadsPerSec *float64 `json:"readsPerSecond,omitempty"`
	// Number of write operations per second
	WritesPerSec *float64 `json:"writesPerSecond,omitempty"`
	// Total number of operations per second
	TotalOpsPerSec *float64 `json:"totalOpsPerSecond,omitempty"`
	// NFS version (will be either 3.0 or 4.0)
	Version *string `json:"version,omitempty"`
	// Device name
	Device *string `json:"device,omitempty"`
	// Mount point of NFS volume
	Mountpoint *string `json:"mountPoint,omitempty"`
	// Filesystem type; used for filtering out non-NFS filesystems
	FilesystemType *string `json:"filesystemType,omitempty"`

	DetailedSample
}

type DetailedSample struct {
	// Age in seconds of NFS client
	Age *float64 `json:"ageSeconds,omitempty"`
	// Number of times cached inode attributes are re-validated from the server.
	InodeRevalidate *uint64 `json:"inodeRevalidate,omitempty"`
	// Number of times cached dentry nodes are re-validated from the server.
	DnodeRevalidate *uint64 `json:"dnodeRevalidate,omitempty"`
	// Number of times an inode cache is cleared.
	DataInvalidate *uint64 `json:"dataInvalidate,omitempty"`
	// Number of times cached inode attributes are invalidated.
	AttributeInvalidate *uint64 `json:"attributeInvalidate,omitempty"`
	// Number of times files or directories have been open()'d.
	VFSOpen *uint64 `json:"vfsOpen,omitempty"`
	// Number of times a directory lookup has occurred.
	VFSLookup *uint64 `json:"vfsLookUp,omitempty"`
	// Number of times permissions have been checked.
	VFSAccess *uint64 `json:"vfsAccess,omitempty"`
	// Number of updates (and potential writes) to pages.
	VFSUpdatePage *uint64 `json:"vfsUpdatePage,omitempty"`
	// Number of pages read directly via mmap()'d files.
	VFSReadPage *uint64 `json:"vfsReadPage,omitempty"`
	// Number of times a group of pages have been read.
	VFSReadPages *uint64 `json:"vfsReadPages,omitempty"`
	// Number of pages written directly via mmap()'d files.
	VFSWritePage *uint64 `json:"vfsWritePage,omitempty"`
	// Number of times a group of pages have been written.
	VFSWritePages *uint64 `json:"vfsWritePages,omitempty"`
	// Number of times directory entries have been read with getdents().
	VFSGetdents *uint64 `json:"vfsGetDents,omitempty"`
	// Number of times attributes have been set on inodes.
	VFSSetattr *uint64 `json:"vfsSetattr,omitempty"`
	// Number of pending writes that have been forcefully flushed to the server.
	VFSFlush *uint64 `json:"vfsFlush,omitempty"`
	// Number of times fsync() has been called on directories and files.
	VFSFsync *uint64 `json:"vfsFsync,omitempty"`
	// Number of times locking has been attempted on a file.
	VFSLock *uint64 `json:"vfsLock,omitempty"`
	// Number of times files have been closed and released.
	VFSFileRelease *uint64 `json:"vfsFileRelease,omitempty"`
	// Number of times files have been truncated.
	Truncation *uint64 `json:"truncation,omitempty"`
	// Number of times a file has been grown due to writes beyond its existing end.
	WriteExtension *uint64 `json:"writeExtension,omitempty"`
	// Number of times a file was removed while still open by another process.
	SillyRename *uint64 `json:"sillyRename,omitempty"`
	// Number of times the NFS server gave less data than expected while reading.
	ShortRead *uint64 `json:"shortRead,omitempty"`
	// Number of times the NFS server wrote less data than expected while writing.
	ShortWrite *uint64 `json:"shortWrite,omitempty"`
	// Number of times the NFS server indicated EJUKEBOX; retrieving data from
	// offline storage.
	JukeboxDelay *uint64 `json:"jukeboxDelay,omitempty"`
	// Number of NFS v4.1+ pNFS reads.
	PNFSRead *uint64 `json:"pnfsRead,omitempty"`
	// Number of NFS v4.1+ pNFS writes.
	PNFSWrite *uint64 `json:"pnfsWrite,omitempty"`
	// Number of times the client has had to establish a connection from scratch
	// to the NFS server.
	Bind *uint64 `json:"bind,omitempty"`
	// Number of times the client has made a TCP connection to the NFS server.
	Connect *uint64 `json:"connect,omitempty"`
	// Duration (in jiffies, a kernel internal unit of time) the NFS mount has
	// spent waiting for connections to the server to be established.
	ConnectIdleTime *uint64 `json:"connectIdleTime,omitempty"`
	// Duration since the NFS mount last saw any RPC traffic.
	IdleTimeSeconds *uint64 `json:"idleTimeSeconds,omitempty"`
	// Number of RPC requests for this mount sent to the NFS server.
	Sends *uint64 `json:"send,omitempty"`
	// Number of RPC responses for this mount received from the NFS server.
	Receives *uint64 `json:"receive,omitempty"`
	// Number of times the NFS server sent a response with a transaction ID
	// unknown to this client.
	BadTransactionIDs *uint64 `json:"badTransactionIds,omitempty"`
	// A running er, incremented on each request as the current difference
	// between sends and receives.
	CumulativeActiveRequests *uint64 `json:"cumulativeActiveRequest,omitempty"`
	// A running counter, incremented on each request by the current backlog
	// queue size.
	CumulativeBacklog *uint64 `json:"cumulativeBacklog,omitempty"`
	// Maximum number of simultaneously active RPC requests ever used.
	MaximumRPCSlotsUsed *uint64 `json:"maximumRPCSlotsUsed,omitempty"`
	// A running counter, incremented on each request as the current size of the
	// sending queue.
	CumulativeSendingQueue *uint64 `json:"cumulativeSendingQueue,omitempty"`
	// A running counter, incremented on each request as the current size of the
	// pending queue.
	CumulativePendingQueue *uint64 `json:"cumulativePendingQueue,omitempty"`
}

func (s *Sampler) OnStartup() {}

func (s *Sampler) Name() string {
	return "NFSSampler"
}

func (s *Sampler) Interval() time.Duration {
	return s.sampleRate
}

func (s *Sampler) Disabled() bool {
	return s.Interval() <= config.FREQ_DISABLE_SAMPLING
}

func (s *Sampler) Sample() (eventBatch sample.EventBatch, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			err = fmt.Errorf("Panic in nfs.Sampler: %v\nStack: %s", panicErr, debug.Stack())
		}
	}()
	samples, err := populateNFS(s.lastSamples, s.detailed)
	if err != nil {
		sslog.WithError(err).Debug("Unable to retrieve NFS stats.")
		return nil, nil
	}
	for _, ss := range samples {
		ss.Type("NFSSample")
		eventBatch = append(eventBatch, ss)
	}
	return eventBatch, err
}

func NewSampler(context agent.AgentContext) *Sampler {
	sampleRateSec := config.DefaultMetricsNFSSampleRate
	detailed := false
	if context != nil {
		sampleRateSec = context.Config().MetricsNFSSampleRate
		detailed = context.Config().DetailedNFS
	}

	return &Sampler{
		context:     context,
		lastSamples: map[string]statsCache{},
		sampleRate:  time.Second * time.Duration(sampleRateSec),
		detailed:    detailed,
	}
}
