package gocql

import "runtime/debug"

const (
	mainModule = "github.com/gocql/gocql"
)

var defaultDriverVersion string

func init() {
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		for _, d := range buildInfo.Deps {
			if d.Path == mainModule {
				defaultDriverVersion = d.Version
				if d.Replace != nil {
					defaultDriverVersion = d.Replace.Version
				}
				break
			}
		}
	}
}
