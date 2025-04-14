package torrc

import (
	"fmt"
	"strings"

	"github.com/fulviodenza/torproxy/api/v1beta1"
)

type torrcOption struct {
	value    any
	required bool
}

func GenerateTorrc(spec v1beta1.TorBridgeConfigSpec) string {
	configs := map[string]torrcOption{
		"Log notice":                {value: "stdout", required: true},
		"ORPort":                    {value: spec.OrPort, required: true},
		"DirPort":                   {value: spec.DirPort, required: false},
		"SOCKSPort":                 {value: fmt.Sprintf("0.0.0.0:%d", spec.SOCKSPort), required: false},
		"BridgeRelay":               {value: 1, required: true},
		"ExitPolicy":                {value: "reject *:*", required: true},
		"ServerTransportPlugin":     {value: spec.ServerTransportPlugin, required: false},
		"ServerTransportListenAddr": {value: spec.ServerTransportListenAddr, required: false},
		"ExtORPort":                 {value: spec.ExtOrPort, required: false},
		"ContactInfo":               {value: spec.ContactInfo, required: false},
		"Nickname":                  {value: spec.Nickname, required: false},
	}

	var lines []string
	for key, opt := range configs {
		if !opt.required && isZeroValue(opt.value) {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %v", key, opt.value))
	}

	return strings.Join(lines, "\n") + "\n"
}

func isZeroValue(v interface{}) bool {
	switch v := v.(type) {
	case string:
		return v == ""
	case int, int32, int64:
		return v == 0
	default:
		return v == nil
	}
}
