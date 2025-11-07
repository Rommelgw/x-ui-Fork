package master

import "time"

type RegisterNodePayload struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MasterURL   string `json:"master_url"`
	IPAddress   string `json:"ip_address"`
	Hostname    string `json:"hostname"`
	Location    string `json:"location"`
	XrayVersion string `json:"xray_version"`
	ListenAddr  string `json:"listen_addr"`
}

type NodeConfigResponse struct {
	Inbounds      []map[string]any `json:"inbounds"`
	Outbounds     []map[string]any `json:"outbounds"`
	Clients       []map[string]any `json:"clients"`
	Routing       map[string]any   `json:"routing,omitempty"`
	DNS           map[string]any   `json:"dns,omitempty"`
	Policy        map[string]any   `json:"policy,omitempty"`
	Transport     map[string]any   `json:"transport,omitempty"`
	Log           map[string]any   `json:"log,omitempty"`
	LastUpdatedAt time.Time        `json:"last_updated_at"`
}

type NodeStatsPayload struct {
	Status      string              `json:"status"`
	CPUUsage    float64             `json:"cpu_usage"`
	MemoryUsage float64             `json:"memory_usage"`
	OnlineUsers int                 `json:"online_users"`
	ClientStats []ClientStatPayload `json:"clients"`
}

type ClientStatPayload struct {
	ClientID uint       `json:"client_id"`
	Upload   int64      `json:"upload"`
	Download int64      `json:"download"`
	LastUsed *time.Time `json:"last_used"`
}
