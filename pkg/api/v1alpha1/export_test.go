// Copyright (C) 2017 ScyllaDB

package v1alpha1

func CheckValues(c *Cluster) error {
	return checkValues(c)
}

func CheckTransitions(old, new *Cluster) error {
	return checkTransitions(old, new)
}
