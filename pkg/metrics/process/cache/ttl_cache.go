package cache

import (
	agentProc "github.com/newrelic/infrastructure-agent/pkg/metrics/process"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/process"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ClockTicks = 100 // C.sysconf(C._SC_CLK_TCK)
)

// cache in-memory cache not to call ps for every process
type cache struct {
	ttl time.Duration
	sync.Mutex
	items     map[int32]psItem
	createdAt time.Time
}

func (c *cache) expired() bool {
	return c == nil || c.createdAt.IsZero() || time.Since(c.createdAt) > c.ttl
}

func (c *cache) update(items map[int32]psItem) {
	c.items = items
	c.createdAt = time.Now()
}

// psItem stores the information of a process and implements process.Process
type psItem struct {
	pid        int32
	ppid       int32
	numThreads int32
	username   string
	state      []string
	command    string
	cmdLine    string
	utime      string
	stime      string
	etime      string
	rss        int64
	vsize      int64
	pagein     int64
}

func (p *psItem) Username() (string, error) {
	return p.username, nil
}

func (p *psItem) Name() (string, error) {
	return p.command, nil
}

func (p *psItem) Cmdline() (string, error) {
	return p.cmdLine, nil
}

func (p *psItem) ProcessId() int32 {
	return p.pid
}

func (p *psItem) Parent() (agentProc.Process, error) {
	return &psItem{pid: p.ppid}, nil
}

func (p *psItem) NumThreads() (int32, error) {
	return p.numThreads, nil
}

func (p *psItem) Status() ([]string, error) {
	return p.state, nil
}

func (p *psItem) MemoryInfo() (*process.MemoryInfoStat, error) {
	return &process.MemoryInfoStat{
		RSS:  uint64(p.rss) * 1024,
		VMS:  uint64(p.vsize) * 1024,
		Swap: uint64(p.pagein),
	}, nil
}

// CPUPercent  returns how many percent of the CPU time this process uses
// it is a c&p of gopsutil process.CPUPercent
func (p *psItem) CPUPercent() (float64, error) {
	crt_time, err := createTime(p.etime)
	if err != nil {
		return 0, err
	}

	cput, err := p.Times()
	if err != nil {
		return 0, err
	}

	created := time.Unix(0, crt_time*int64(time.Millisecond))
	totalTime := time.Since(created).Seconds()
	if totalTime <= 0 {
		return 0, nil
	}

	return 100 * cput.Total() / totalTime, nil
}

func (p *psItem) Times() (*cpu.TimesStat, error) {
	return times(p.utime, p.stime)
}

// createTime retrieves ceate time from ps output etime field
// it is a c&p of gopsutil process.CreateTimeWithContext
func createTime(etime string) (int64, error) {
	elapsedSegments := strings.Split(strings.Replace(etime, "-", ":", 1), ":")
	var elapsedDurations []time.Duration
	for i := len(elapsedSegments) - 1; i >= 0; i-- {
		p, err := strconv.ParseInt(elapsedSegments[i], 10, 0)
		if err != nil {
			return 0, err
		}
		elapsedDurations = append(elapsedDurations, time.Duration(p))
	}

	var elapsed = elapsedDurations[0] * time.Second
	if len(elapsedDurations) > 1 {
		elapsed += elapsedDurations[1] * time.Minute
	}
	if len(elapsedDurations) > 2 {
		elapsed += elapsedDurations[2] * time.Hour
	}
	if len(elapsedDurations) > 3 {
		elapsed += elapsedDurations[3] * time.Hour * 24
	}

	start := time.Now().Add(-elapsed)
	return start.Unix() * 1000, nil
}

// times retrieves ceate time from ps output utime and stime fields
// it is a c&p of gopsutil process.TimesWithContext
func times(utime string, stime string) (*cpu.TimesStat, error) {
	uCpuTimes, err := convertCPUTimes(utime)
	if err != nil {
		return nil, err
	}
	sCpuTimes, err := convertCPUTimes(stime)
	if err != nil {
		return nil, err
	}

	ret := &cpu.TimesStat{
		CPU:    "cpu",
		User:   uCpuTimes,
		System: sCpuTimes,
	}
	return ret, nil
}

// convertCPUTimes converts ps format cputime to time units that are in USER_HZ or Jiffies
// it is a c&p of gopsutil process.convertCPUTimes
func convertCPUTimes(s string) (ret float64, err error) {
	var t int
	var _tmp string
	if strings.Contains(s, ":") {
		_t := strings.Split(s, ":")
		hour, err := strconv.Atoi(_t[0])
		if err != nil {
			return ret, err
		}
		t += hour * 60 * 100
		_tmp = _t[1]
	} else {
		_tmp = s
	}

	_t := strings.Split(_tmp, ".")
	if err != nil {
		return ret, err
	}
	h, err := strconv.Atoi(_t[0])
	t += h * 100
	h, err = strconv.Atoi(_t[1])
	t += h
	return float64(t) / ClockTicks, nil
}
