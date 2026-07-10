package app

import "fmt"

type Config struct {
	Addr          string
	DatabasePath  string
	OperatorToken string
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
	return nil
}
