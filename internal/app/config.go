package app

import "fmt"

type Config struct {
	Addr              string
	DatabasePath      string
	OperatorToken     string
	JWTSecret         string
	PwnedPasswordsURL string
	STUNServers       []string
	TURNServers       []ICEServer
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func (c Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("addr is required")
	}
	if c.DatabasePath == "" {
		return fmt.Errorf("database path is required")
	}
	if c.OperatorToken == "" {
		return fmt.Errorf("operator token is required")
	}
	if c.JWTSecret == "" {
		return fmt.Errorf("jwt secret is required")
	}
	if len(c.STUNServers) == 0 {
		return fmt.Errorf("at least one STUN server is required")
	}
	return nil
}

func (c Config) ICEConfig() []ICEServer {
	servers := []ICEServer{{URLs: c.STUNServers}}
	for _, server := range c.TURNServers {
		if len(server.URLs) > 0 {
			servers = append(servers, server)
		}
	}
	return servers
}
