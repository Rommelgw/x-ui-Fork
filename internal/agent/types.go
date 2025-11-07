package agent

import "time"

type GenericMap map[string]any

type ClientConfig GenericMap

type SyncRequest struct {
	Action    string     `json:"action"`
	Config    XrayConfig `json:"config"`
	Timestamp int64      `json:"timestamp"`
}

type XrayConfig struct {
	Inbounds      []GenericMap   `json:"inbounds"`
	Outbounds     []GenericMap   `json:"outbounds"`
	Clients       []ClientConfig `json:"clients"`
	Routing       GenericMap     `json:"routing,omitempty"`
	DNS           GenericMap     `json:"dns,omitempty"`
	Policy        GenericMap     `json:"policy,omitempty"`
	Transport     GenericMap     `json:"transport,omitempty"`
	Log           GenericMap     `json:"log,omitempty"`
	OtherSections GenericMap     `json:"other_sections,omitempty"`
}

type RuntimeConfig struct {
	Inbounds  []GenericMap
	Outbounds []GenericMap
	Clients   []ClientConfig
	Routing   GenericMap
	DNS       GenericMap
	Policy    GenericMap
	Transport GenericMap
	Log       GenericMap
	Extras    GenericMap
}

type NodeStats struct {
	Status      string           `json:"status"`
	CPUUsage    float64          `json:"cpu_usage"`
	MemoryUsage float64          `json:"memory_usage"`
	OnlineUsers int              `json:"online_users"`
	ClientStats []NodeClientStat `json:"clients"`
}

type NodeClientStat struct {
	ClientID uint       `json:"client_id"`
	Upload   int64      `json:"upload"`
	Download int64      `json:"download"`
	LastUsed *time.Time `json:"last_used"`
}
