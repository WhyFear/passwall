package model

import (
	"strconv"
	"strings"
)

// RuntimeName encodes the database identity used by Mihomo traffic snapshots.
func (p *Proxy) RuntimeName(index int) string {
	name := "[pw:" + strconv.FormatUint(uint64(p.ID), 10) + "]-"
	if index > 0 {
		name += "[" + strconv.Itoa(index) + "]-"
	}
	return name + p.Name
}

// ParseRuntimeProxyID extracts only IDs emitted by RuntimeName.
func ParseRuntimeProxyID(name string) (uint, bool) {
	if !strings.HasPrefix(name, "[pw:") {
		return 0, false
	}
	end := strings.IndexByte(name, ']')
	if end < 5 || len(name) <= end+1 || name[end+1] != '-' {
		return 0, false
	}
	id, err := strconv.ParseUint(name[4:end], 10, 0)
	return uint(id), err == nil && id > 0
}
