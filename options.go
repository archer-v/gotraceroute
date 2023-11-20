package traceroute

import "time"

const DefaultPort = 33434
const DefaultMaxHops = 32
const DefaultStartTTL = 1
const DefaultTimeoutMs = 200
const DefaultRetries = 2

const maxHopsLimit = 63

// Options type
type Options struct {
	Port             int
	MaxHops          int
	StartTTL         int
	Timeout          time.Duration
	Retries          int
	PayloadSize      int
	NetworkInterface string
	DontResolve      bool
}

func (o *Options) port() int {
	if o.Port == 0 {
		o.Port = DefaultPort
	}
	return o.Port
}

func (o *Options) maxHops() int {
	if o.MaxHops == 0 {
		o.MaxHops = DefaultMaxHops
	}
	if o.MaxHops > maxHopsLimit {
		o.MaxHops = maxHopsLimit
	}
	return o.MaxHops
}

func (o *Options) startTTL() int {
	if o.StartTTL == 0 {
		o.StartTTL = DefaultStartTTL
	}
	return o.StartTTL
}

func (o *Options) timeout() time.Duration {
	if o.Timeout == 0 {
		o.Timeout = time.Millisecond * DefaultTimeoutMs
	}
	return o.Timeout
}

func (o *Options) retries() int {
	if o.Retries == 0 {
		o.Retries = DefaultRetries
	}
	return o.Retries
}

func (o *Options) payloadSize() int {
	return o.PayloadSize
}
