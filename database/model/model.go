// Package model defines the database models and data structures used by the 3x-ui panel.
package model

import (
	"fmt"

	"github.com/mhsanaei/3x-ui/v2/util/json_util"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

// Protocol represents the protocol type for Xray inbounds.
type Protocol string

// Protocol constants for different Xray inbound protocols
const (
	VMESS       Protocol = "vmess"
	VLESS       Protocol = "vless"
	Tunnel      Protocol = "tunnel"
	HTTP        Protocol = "http"
	Trojan      Protocol = "trojan"
	Shadowsocks Protocol = "shadowsocks"
	Mixed       Protocol = "mixed"
	WireGuard   Protocol = "wireguard"
)

// User represents a user account in the 3x-ui panel.
type User struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Inbound represents an Xray inbound configuration with traffic statistics and settings.
type Inbound struct {
	Id                   int                  `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`                                                    // Unique identifier
	UserId               int                  `json:"-"`                                                                                               // Associated user ID
	Up                   int64                `json:"up" form:"up"`                                                                                    // Upload traffic in bytes
	Down                 int64                `json:"down" form:"down"`                                                                                // Download traffic in bytes
	Total                int64                `json:"total" form:"total"`                                                                              // Total traffic limit in bytes
	AllTime              int64                `json:"allTime" form:"allTime" gorm:"default:0"`                                                         // All-time traffic usage
	Remark               string               `json:"remark" form:"remark"`                                                                            // Human-readable remark
	Enable               bool                 `json:"enable" form:"enable" gorm:"index:idx_enable_traffic_reset,priority:1"`                           // Whether the inbound is enabled
	ExpiryTime           int64                `json:"expiryTime" form:"expiryTime"`                                                                    // Expiration timestamp
	TrafficReset         string               `json:"trafficReset" form:"trafficReset" gorm:"default:never;index:idx_enable_traffic_reset,priority:2"` // Traffic reset schedule
	LastTrafficResetTime int64                `json:"lastTrafficResetTime" form:"lastTrafficResetTime" gorm:"default:0"`                               // Last traffic reset timestamp
	ClientStats          []xray.ClientTraffic `gorm:"foreignKey:InboundId;references:Id" json:"clientStats" form:"clientStats"`                        // Client traffic statistics

	// Xray configuration fields
	Listen         string   `json:"listen" form:"listen"`
	Port           int      `json:"port" form:"port"`
	Protocol       Protocol `json:"protocol" form:"protocol"`
	Settings       string   `json:"settings" form:"settings"`
	StreamSettings string   `json:"streamSettings" form:"streamSettings"`
	Tag            string   `json:"tag" form:"tag" gorm:"unique"`
	Sniffing       string   `json:"sniffing" form:"sniffing"`
}

// OutboundTraffics tracks traffic statistics for Xray outbound connections.
type OutboundTraffics struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Tag   string `json:"tag" form:"tag" gorm:"unique"`
	Up    int64  `json:"up" form:"up" gorm:"default:0"`
	Down  int64  `json:"down" form:"down" gorm:"default:0"`
	Total int64  `json:"total" form:"total" gorm:"default:0"`
}

// InboundClientIps stores IP addresses associated with inbound clients for access control.
type InboundClientIps struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ClientEmail string `json:"clientEmail" form:"clientEmail" gorm:"unique"`
	Ips         string `json:"ips" form:"ips"`
}

// HistoryOfSeeders tracks which database seeders have been executed to prevent re-running.
type HistoryOfSeeders struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SeederName string `json:"seederName"`
}

// GenXrayInboundConfig generates an Xray inbound configuration from the Inbound model.
func (i *Inbound) GenXrayInboundConfig() *xray.InboundConfig {
	listen := i.Listen
	if listen != "" {
		listen = fmt.Sprintf("\"%v\"", listen)
	}
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       string(i.Protocol),
		Settings:       json_util.RawMessage(i.Settings),
		StreamSettings: json_util.RawMessage(i.StreamSettings),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
}

// Setting stores key-value configuration settings for the 3x-ui panel.
type Setting struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

// Client represents a client configuration for Xray inbounds with traffic limits and settings.
type Client struct {
	ID         string `json:"id"`                           // Unique client identifier
	Security   string `json:"security"`                     // Security method (e.g., "auto", "aes-128-gcm")
	Password   string `json:"password"`                     // Client password
	Flow       string `json:"flow"`                         // Flow control (XTLS)
	Email      string `json:"email"`                        // Client email identifier
	LimitIP    int    `json:"limitIp"`                      // IP limit for this client
	TotalGB    int64  `json:"totalGB" form:"totalGB"`       // Total traffic limit in GB
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"` // Expiration timestamp
	Enable     bool   `json:"enable" form:"enable"`         // Whether the client is enabled
	TgID       int64  `json:"tgId" form:"tgId"`             // Telegram user ID for notifications
	SubID      string `json:"subId" form:"subId"`           // Subscription identifier
	Comment    string `json:"comment" form:"comment"`       // Client comment
	Reset      int    `json:"reset" form:"reset"`           // Reset period in days
	CreatedAt  int64  `json:"created_at,omitempty"`         // Creation timestamp
	UpdatedAt  int64  `json:"updated_at,omitempty"`         // Last update timestamp
}

// NodeStatus represents the status of a node
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusError   NodeStatus = "error"
)

// Node represents a remote 3x-ui server node that can be managed from the master panel.
type Node struct {
	Id          int        `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`                        // Unique identifier
	Name        string     `json:"name" form:"name"`                                                    // Node name
	Host        string     `json:"host" form:"host"`                                                    // IP address or domain
	Port        int        `json:"port" form:"port"`                                                    // API port
	ApiKey      string     `json:"apiKey" form:"apiKey"`                                                // API key for authentication
	Protocol    string     `json:"protocol" form:"protocol" gorm:"default:https"`                       // http or https
	Location    string     `json:"location" form:"location"`                                            // Location name (e.g., "Moscow")
	Country     string     `json:"country" form:"country"`                                              // Country code (ISO 3166-1 alpha-2)
	City        string     `json:"city" form:"city"`                                                    // City name
	Latitude    float64    `json:"latitude" form:"latitude"`                                            // Latitude coordinate
	Longitude   float64    `json:"longitude" form:"longitude"`                                          // Longitude coordinate
	Enable      bool       `json:"enable" form:"enable" gorm:"default:true"`                            // Whether the node is enabled
	Status      NodeStatus `json:"status" form:"status" gorm:"default:offline"`                         // Node status: online, offline, error
	LastCheck   int64      `json:"lastCheck" form:"lastCheck" gorm:"default:0"`                         // Last status check timestamp
	Remark      string     `json:"remark" form:"remark"`                                                // Remark/notes
	CreatedAt   int64      `json:"createdAt" form:"createdAt" gorm:"autoCreateTime"`                    // Creation timestamp
	UpdatedAt   int64      `json:"updatedAt" form:"updatedAt" gorm:"autoUpdateTime"`                    // Last update timestamp
}

// MultiSubscription represents a subscription that combines multiple nodes.
type MultiSubscription struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`     // Unique identifier
	Name      string `json:"name" form:"name"`                                 // Subscription name
	SubId     string `json:"subId" form:"subId" gorm:"uniqueIndex"`            // Unique subscription ID
	NodeIds   string `json:"nodeIds" form:"nodeIds"`                           // JSON array of node IDs
	Enable    bool   `json:"enable" form:"enable" gorm:"default:true"`         // Whether the subscription is enabled
	Remark    string `json:"remark" form:"remark"`                             // Remark/notes
	CreatedAt int64  `json:"createdAt" form:"createdAt" gorm:"autoCreateTime"` // Creation timestamp
	UpdatedAt int64  `json:"updatedAt" form:"updatedAt" gorm:"autoUpdateTime"` // Last update timestamp
}

// NodeStats represents statistics collected from a node.
type NodeStats struct {
	Id          int     `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`           // Unique identifier
	NodeId      int     `json:"nodeId" form:"nodeId" gorm:"index"`                      // Node ID
	Cpu         float64 `json:"cpu" form:"cpu" gorm:"default:0"`                        // CPU usage percentage
	Mem         uint64  `json:"mem" form:"mem" gorm:"default:0"`                        // Memory usage in bytes
	MemTotal    uint64  `json:"memTotal" form:"memTotal" gorm:"default:0"`              // Total memory in bytes
	Disk        uint64  `json:"disk" form:"disk" gorm:"default:0"`                      // Disk usage in bytes
	DiskTotal   uint64  `json:"diskTotal" form:"diskTotal" gorm:"default:0"`            // Total disk space in bytes
	NetUp       uint64  `json:"netUp" form:"netUp" gorm:"default:0"`                    // Network traffic UP in bytes
	NetDown     uint64  `json:"netDown" form:"netDown" gorm:"default:0"`                // Network traffic DOWN in bytes
	Uptime      uint64  `json:"uptime" form:"uptime" gorm:"default:0"`                  // Uptime in seconds
	XrayStatus  string  `json:"xrayStatus" form:"xrayStatus" gorm:"default:stop"`       // Xray status: running, stop, error
	Clients     int     `json:"clients" form:"clients" gorm:"default:0"`                // Number of active clients
	Inbounds    int     `json:"inbounds" form:"inbounds" gorm:"default:0"`              // Number of inbounds
	CollectedAt int64   `json:"collectedAt" form:"collectedAt" gorm:"autoCreateTime"`   // Statistics collection timestamp
}
