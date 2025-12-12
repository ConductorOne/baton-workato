package main

import (
	"github.com/conductorone/baton-sdk/pkg/config"
	cfg "github.com/conductorone/baton-workato/pkg/config"
)

func main() {
	config.Generate("workato", cfg.Config)
}
