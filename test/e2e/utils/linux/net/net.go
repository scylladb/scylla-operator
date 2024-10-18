// Copyright (c) 2024 ScyllaDB.

package net

import (
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/scylladb/scylla-operator/pkg/helpers/slices"
)

const (
	procNetEntryRegexLocalIP    = "local_ip"
	procNetEntryRegexLocalPort  = "local_port"
	procNetEntryRegexRemoteIP   = "remote_ip"
	procNetEntryRegexRemotePort = "remote_port"
	procNetEntryRegexState      = "type"
)

var (
	procNetEntryRegex = regexp.MustCompile(`^(\s+)?\d+:\s+(?P<local_ip>[0-9A-F]{8,}):(?P<local_port>[0-9A-F]{4})\s+(?P<remote_ip>[0-9A-F]{8,}):(?P<remote_port>[0-9A-F]{4})\s+(?P<type>[0-9A-F]{2})\s+`)
)

const (
	_ = iota
	entryStateEstablishedIndex
	entryStateSentIndex
	entryStateReceivedIndex
	entryStateFinWait1Index
	entryStateFinWait2Index
	entryStateTimeWaitIndex
	entryStateCloseIndex
	entryStateCloseWaitIndex
	entryStateLastAckIndex
	entryStateListenIndex
	entryStateClosingIndex
	entryStateNewSynReceivedIndex
	entryStateBoundInactiveIndex
	entryStateMaxStatesIndex
)

type EntryState string

// Comes from net/tcp_states.h
const (
	EntryStateEstablished    EntryState = "Established"
	EntryStateSent           EntryState = "Sent"
	EntryStateReceived       EntryState = "Received"
	EntryStateFinWait1       EntryState = "FinWait1"
	EntryStateFinWait2       EntryState = "FinWait2"
	EntryStateTimeWait       EntryState = "TimeWait"
	EntryStateClose          EntryState = "Close"
	EntryStateCloseWait      EntryState = "CloseWait"
	EntryStateLastAck        EntryState = "LastAck"
	EntryStateListen         EntryState = "Listen"
	EntryStateClosing        EntryState = "Closing"
	EntryStateNewSynReceived EntryState = "NewSynReceived"
	EntryStateBoundInactive  EntryState = "BoundInactive"
	EntryStateMaxStates      EntryState = "MaxStates"
)

func parseEntryState(v int) (EntryState, error) {
	switch v {
	case entryStateEstablishedIndex:
		return EntryStateEstablished, nil
	case entryStateSentIndex:
		return EntryStateSent, nil
	case entryStateReceivedIndex:
		return EntryStateReceived, nil
	case entryStateFinWait1Index:
		return EntryStateFinWait1, nil
	case entryStateFinWait2Index:
		return EntryStateFinWait2, nil
	case entryStateTimeWaitIndex:
		return EntryStateTimeWait, nil
	case entryStateCloseIndex:
		return EntryStateClose, nil
	case entryStateCloseWaitIndex:
		return EntryStateCloseWait, nil
	case entryStateLastAckIndex:
		return EntryStateLastAck, nil
	case entryStateListenIndex:
		return EntryStateListen, nil
	case entryStateClosingIndex:
		return EntryStateClosing, nil
	case entryStateNewSynReceivedIndex:
		return EntryStateNewSynReceived, nil
	case entryStateBoundInactiveIndex:
		return EntryStateBoundInactive, nil
	case entryStateMaxStatesIndex:
		return EntryStateMaxStates, nil
	default:
		return "", fmt.Errorf("can't parse entry state %q", v)
	}
}

type AddressPort struct {
	Address net.IP
	Port    int
}

type ProcNetEntry struct {
	LocalAddress  AddressPort
	RemoteAddress AddressPort
	State         EntryState
}

func reverseBytes(data []byte) []byte {
	res := make([]byte, 0, len(data))

	for i := len(data) - 1; i >= 0; i-- {
		res = append(res, data[i])
	}

	return res
}

func decodeIPAddress(hexString string) (net.IP, error) {
	v, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("can't decode address %q: %w", hexString, err)
	}

	l := len(v)
	if l != net.IPv4len && l != net.IPv6len {
		return nil, fmt.Errorf("unexpected address length %d for address %q", l, v)
	}

	return net.IP(reverseBytes(v)).To16(), nil
}

func decodePort(hexString string) (int, error) {
	v, err := strconv.ParseUint(hexString, 16, 16)
	if err != nil {
		return 0, fmt.Errorf("can't parse port %q: %w", hexString, err)
	}

	if v >= (1 << 16) {
		return 0, fmt.Errorf("port %d is out of range", v)
	}

	return int(v), nil
}

func decodeState(hexString string) (*EntryState, error) {
	v, err := strconv.ParseUint(hexString, 16, 16)
	if err != nil {
		return nil, fmt.Errorf("can't parse state %q: %w", hexString, err)
	}

	state, err := parseEntryState(int(v))
	if err != nil {
		return nil, fmt.Errorf("can't parse state %q: %w", v, err)
	}

	return &state, nil
}

func ParseProcNetEntry(line string) (*ProcNetEntry, error) {
	matches := procNetEntryRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("can't parse line %q", line)
	}

	localAddress, err := decodeIPAddress(matches[procNetEntryRegex.SubexpIndex(procNetEntryRegexLocalIP)])
	if err != nil {
		return nil, fmt.Errorf("can't decode local address: %w", err)
	}

	localPort, err := decodePort(matches[procNetEntryRegex.SubexpIndex(procNetEntryRegexLocalPort)])
	if err != nil {
		return nil, fmt.Errorf("can't decode local port: %w", err)
	}

	remoteAddress, err := decodeIPAddress(matches[procNetEntryRegex.SubexpIndex(procNetEntryRegexRemoteIP)])
	if err != nil {
		return nil, fmt.Errorf("can't decode remote address: %w", err)
	}

	remotePort, err := decodePort(matches[procNetEntryRegex.SubexpIndex(procNetEntryRegexRemotePort)])
	if err != nil {
		return nil, fmt.Errorf("can't decode remote port: %w", err)
	}

	state, err := decodeState(matches[procNetEntryRegex.SubexpIndex(procNetEntryRegexState)])
	if err != nil {
		return nil, fmt.Errorf("can't decode state: %w", err)
	}

	return &ProcNetEntry{
		LocalAddress: AddressPort{
			Address: localAddress,
			Port:    localPort,
		},
		RemoteAddress: AddressPort{
			Address: remoteAddress,
			Port:    remotePort,
		},
		State: *state,
	}, nil
}

type ProcNetEntries []ProcNetEntry

func ParseProcNetEntries(data string) (ProcNetEntries, error) {
	var res ProcNetEntries

	lines := strings.Split(strings.Trim(data, "\n"), "\n")
	var procNetEntry *ProcNetEntry
	var err error
	for _, line := range lines {
		procNetEntry, err = ParseProcNetEntry(line)
		if err != nil {
			return nil, fmt.Errorf("can't parse entry state: %w", err)
		}

		res = append(res, *procNetEntry)
	}

	return res, nil
}

func (pe ProcNetEntries) FilterListen() []AddressPort {
	filtered := slices.Filter(pe, func(entry ProcNetEntry) bool {
		return entry.State == EntryStateListen
	})
	return slices.ConvertSlice(filtered, func(from ProcNetEntry) AddressPort {
		return from.LocalAddress
	})
}
