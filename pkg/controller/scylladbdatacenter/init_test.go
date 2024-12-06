package scylladbdatacenter

import (
	"flag"

	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(flag.CommandLine)
	err := flag.Set("alsologtostderr", "true")
	if err != nil {
		panic(err)
	}
}
