package main

import (
	"fmt"
	"strings"
)

type forwarderConfig struct {
	SourceAddr string
	TargetAddr string
}

func newForwarderConfig(configStr string) (forwarderConfig, error) {
	splits := strings.Split(configStr, "->")
	if len(splits) < 2 {
		return forwarderConfig{}, fmt.Errorf("invalid forward config format: %s", configStr)
	}

	return forwarderConfig{
		SourceAddr: strings.TrimSpace(splits[0]),
		TargetAddr: strings.TrimSpace(splits[1]),
	}, nil
}

func parseForwarderConfigList(forwardConfigStringList []string) ([]forwarderConfig, error) {
	var result []forwarderConfig
	for idx, item := range forwardConfigStringList {
		cfg, err := newForwarderConfig(item)
		if err != nil {
			return nil, fmt.Errorf("forward config err[%d]: %w", idx, err)
		}
		result = append(result, cfg)
	}
	return result, nil
}
