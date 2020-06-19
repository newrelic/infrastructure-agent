// Copyright 2020 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package hostname

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/newrelic/infrastructure-agent/pkg/log"
	"github.com/newrelic/infrastructure-agent/pkg/trace"
)

var (
	fullHostnameResolver = getFqdnHostname
	logger               = log.WithComponent("HostnameResolver")
)

// Resolver provides full and short name resolving functionalities
type Resolver interface {
	// Query returns the full and the short hostname, or error if the process has not been completed
	Query() (full, short string, err error)
	Long() string
}

// ChangeType represents the type of hostname change
type ChangeType int

const (
	// Short is the short hostname
	Short ChangeType = iota
	// Full is the FDQN hostname
	Full
	// ShortAndFull both were changed
	ShortAndFull
)

// ChangeNotification is the struct being sent through the notification channel
type ChangeNotification struct {
	What ChangeType
}

// ChangeNotifier allows observer to register a channel to be notified of when the hostname is updated
type ChangeNotifier interface {
	AddObserver(name string, ch chan<- ChangeNotification)
	RemoveObserver(name string)
}

// ResolverChangeNotifier is a sum of both Resolver and ChangeNotifier interfaces
type ResolverChangeNotifier interface {
	Resolver
	ChangeNotifier
}

// CreateResolver creates a HostnameResolver.
// If overrideFull or overrideShort are not empty strings, the hostname won't resolve automatically but will use
// the passed values.
// If dnsResolution is true, returns a HostnameResolver that attempts to resolve the Fully Qualified Domain Name
// as the full hostname.
// If dnsResolution is false, returns a HostnameResolver resolves internally the full hostname (asking to the OS for it).
// If the full hostname resolution process fails (e.g. due to a temporary DNS failure), it
// returns the previous successful resolution (or the short hostname if it has never worked
// previously).
func CreateResolver(overrideFull, overrideShort string, dnsResolution bool) ResolverChangeNotifier {
	var resolver *fallbackResolver
	if dnsResolution {
		resolver = newDNSResolver(overrideFull)
	} else {
		resolver = newInternalResolver(overrideFull)
	}

	resolver.short = os.Hostname
	resolver.observers = map[string]chan<- ChangeNotification{}

	if overrideShort != "" {
		resolver.short = func() (string, error) {
			trace.Hostname("using override_hostname_short property '%s'", overrideShort)
			return overrideShort, nil
		}
	}
	return resolver
}

func newDNSResolver(overrideFull string) *fallbackResolver {
	return &fallbackResolver{
		full:          fullHostnameResolver,
		internal:      internalHostname,
		overridenFull: overrideFull,
	}
}

func newInternalResolver(overrideFull string) *fallbackResolver {
	var fullResolver = func(_ string) (string, error) {
		return internalHostname()
	}
	var internalResolver = func() (string, error) {
		// after fullResolver has failed, this will make the fallback resolver to use the last successful resolution
		return "", errors.New("internal hostname resolution did not work")
	}
	return &fallbackResolver{
		internal:      internalResolver,
		full:          fullResolver,
		overridenFull: overrideFull,
	}
}

// Looks up for the Fully Qualified Domain Name
func getFqdnHostname(osHost string) (string, error) {
	ips, err := net.LookupIP(osHost)
	if err != nil {
		return "", err
	}

	for _, ip := range ips {
		hosts, err := net.LookupAddr(ip.String())
		if err != nil || len(hosts) == 0 {
			return "", err
		}
		trace.Hostname("found FQDN hosts: %s", strings.Join(hosts, ", "))
		return strings.TrimSuffix(hosts[0], "."), nil
	}
	return "", errors.New("can't lookup FQDN")
}

// Implementation of the HostnameResolver interface that provides fallback capabilities
// in case the full name resolving fails, returning the last successful value.
// If the Full name resolution is "localhost" (known problem in some wrong FQDN configurations)
// the "internal" name resolver is applied.
type fallbackResolver struct {
	sync.Mutex
	lastShort     string
	lastFull      string
	overridenFull string
	short         func() (string, error)
	internal      func() (string, error)
	full          func(string) (string, error)
	observers     map[string]chan<- ChangeNotification
}

// Query returns the full and the short host name, or error if none of both can't be returned.
// This implementation assumes the full host name may fail since it can depend on an external
// service (e.g. a DNS server). If the full name resolution fails, it considers the following
// fallback actions (in priority):
// 1 - return the previous successful full name resolution resolution
// 2 - ask for the full hostname to the OS (and consider the returned value as successful)
// 3 - The short host name if it has never been successfully resolved.
func (r *fallbackResolver) Query() (string, string, error) {
	short, err := r.short()
	var full string
	if r.overridenFull != "" {
		trace.Hostname("using override_hostname property '%s'", r.overridenFull)
		full = r.overridenFull
	} else {
		if err != nil {
			trace.Hostname("failed to resolve short hostname: %s", err)
		} else {
			full, err = r.full(short)
		}
		// Fixes some wrong FQDN configurations that return "localhost". We bypass the FQDN resolution and cache
		// and just return the full hostname as queried by the kernel (the old behaviour of the agent)
		if r.lastFull == "" && (full == "" || full == "localhost") {
			// In this edge case, the hostname could flip under some network name instability circumstances
			trace.Hostname("using internal hostname")
			full, err = r.internal()
			if err != nil {
				trace.Hostname("internal hostname resolution failed")
			}
			if full == "localhost" {
				full = ""
			}
		}
	}
	return r.updateAndGet(full, short, err)
}

func (r *fallbackResolver) updateAndGet(queriedFull, queriedShort string, cause error) (full, short string, err error) {
	var shouldNotify bool
	var what ChangeType
	// only change if different
	if queriedShort != "" && r.lastShort != queriedShort {
		r.lastShort = queriedShort
		shouldNotify = true
		what = Short
	}

	// only change if different
	if queriedFull != "" && r.lastFull != queriedFull {
		r.lastFull = queriedFull
		shouldNotify = true
		if what == Short {
			what = ShortAndFull
		} else {
			what = Full
		}

	}

	if r.lastFull == "" {
		full = r.lastShort
	} else {
		full = r.lastFull
	}

	if r.lastShort == "" && full == "" {
		err = fmt.Errorf("can't query neither full nor short hostname: %v", cause)
	}

	// this is to avoid loops of query->update->query because we update when we query which is not a very good idea...
	// fix this query side-effect later
	if shouldNotify && err == nil {
		r.notifyObservers(what)
	}

	return full, r.lastShort, err
}

func (r *fallbackResolver) Long() string {
	if r.lastFull == "" {
		_, _, _ = r.Query()
	}

	return r.lastFull
}

func (r *fallbackResolver) AddObserver(name string, ch chan<- ChangeNotification) {
	r.Lock()
	defer r.Unlock()

	r.observers[name] = ch
	logger.WithField("name", name).WithField("num", len(r.observers)).Debug("Observer added.")
}

func (r *fallbackResolver) RemoveObserver(name string) {
	r.Lock()
	defer r.Unlock()

	delete(r.observers, name)
	logger.WithField("name", name).WithField("num", len(r.observers)).Debug("Observer removed.")
}

func (r *fallbackResolver) notifyObservers(change ChangeType) {
	// copy map so we don't change while iterating
	observers := make(map[string]chan<- ChangeNotification)
	r.Lock()
	for name, ch := range r.observers {
		observers[name] = ch
	}
	r.Unlock()

	logger.WithField("change", change).Debug("Notifying observers.")
	for name, ch := range observers {
		// don't block while trying to write
		select {
		case ch <- ChangeNotification{What: change}:
			logger.WithField("name", name).Debug("Observer notified.")
		default:
		}
	}
}
