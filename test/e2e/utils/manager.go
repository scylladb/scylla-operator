package utils

import "github.com/scylladb/scylla-operator/test/e2e/framework"

// LocationForScyllaManagerWithDC returns a `<dc>:<provider>:<location>` string for Scylla Manager configuration.
func LocationForScyllaManagerWithDC(s framework.ClusterObjectStorageSettings, dc string) string {
	return dc + ":" + LocationForScyllaManager(s)
}

// LocationForScyllaManager returns a `<provider>:<location>` string for Scylla Manager configuration.
func LocationForScyllaManager(s framework.ClusterObjectStorageSettings) string {
	switch s.Type() {
	case framework.ObjectStorageTypeGCS:
		return "gcs:" + s.BucketName()
	case framework.ObjectStorageTypeS3:
		return "s3:" + s.BucketName()
	default:
		return ""
	}
}
