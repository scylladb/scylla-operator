// Copyright (C) 2017 ScyllaDB

package v1

func CheckValues(c *ScyllaCluster) error {
	return checkValues(c)
}

func CheckTransitions(old, new *ScyllaCluster) error {
	return checkTransitions(old, new)
}
