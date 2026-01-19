/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
/*
 * Content before git sha 34fdeebefcbf183ed7f916f931aa0586fdaa1b40
 * Copyright (c) 2016, The Gocql authors,
 * provided under the BSD-3-Clause License.
 * See the NOTICE file distributed with this work for additional information.
 */

package gocql

import "net"

// AddressTranslator provides a way to translate node addresses (and ports) that are
// discovered or received as a node event. This can be useful in an ec2 environment,
// for instance, to translate public IPs to private IPs.
type AddressTranslator interface {
	// Translate will translate the provided address and/or port to another
	// address and/or port. If no translation is possible, Translate will return the
	// address and port provided to it.
	Translate(addr net.IP, port int) (net.IP, int)
}

type AddressTranslatorFunc func(addr net.IP, port int) (net.IP, int)

func (fn AddressTranslatorFunc) Translate(addr net.IP, port int) (net.IP, int) {
	return fn(addr, port)
}

// IdentityTranslator will do nothing but return what it was provided. It is essentially a no-op.
func IdentityTranslator() AddressTranslator {
	return AddressTranslatorFunc(func(addr net.IP, port int) (net.IP, int) {
		return addr, port
	})
}

type AddressTranslatorHostInfo interface {
	HostID() string
	Rack() string
	DataCenter() string
	BroadcastAddress() net.IP
	ListenAddress() net.IP
	RPCAddress() net.IP
	PreferredIP() net.IP
	Peer() net.IP
	UntranslatedConnectAddress() net.IP
	Port() int
	Partitioner() string
	ClusterName() string
	ScyllaShardAwarePort() uint16
	ScyllaShardAwarePortTLS() uint16
	ScyllaShardCount() int
}

// AddressTranslatorV2 provides a way to translate node addresses (and ports) that are
// discovered or received as a node event. This can be useful in an ec2 environment,
// for instance, to translate public IPs to private IPs.
type AddressTranslatorV2 interface {
	AddressTranslator
	TranslateHost(host AddressTranslatorHostInfo, addr AddressPort) (AddressPort, error)
}

type AddressTranslatorFuncV2 func(hostID string, addr AddressPort) AddressPort

func (fn AddressTranslatorFuncV2) Translate(addr net.IP, port int) (net.IP, int) {
	res := fn("", AddressPort{
		Address: addr,
		Port:    uint16(port),
	})
	return res.Address, int(res.Port)
}

func (fn AddressTranslatorFuncV2) TranslateHost(host AddressTranslatorHostInfo, addr AddressPort) (AddressPort, error) {
	return fn(host.HostID(), addr), nil
}

var _ AddressTranslatorV2 = AddressTranslatorFuncV2(nil)
