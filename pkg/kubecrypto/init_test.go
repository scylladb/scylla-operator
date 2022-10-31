package kubecrypto

import (
	"flag"

	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(flag.CommandLine)
	err := flag.Set("logtostderr", "true")
	if err != nil {
		panic(err)
	}

	// Default log level to 2.
	levelPtr := (*int32)(flag.CommandLine.Lookup("v").Value.(*klog.Level))
	*levelPtr = int32(2)
}
