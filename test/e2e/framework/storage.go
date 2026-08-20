package framework

import (
	"errors"
)

// ClusterObjectStorageSettings holds the object storage settings for a cluster.
type ClusterObjectStorageSettings struct {
	// storageType is the type of object storage used by the cluster.
	storageType ObjectStorageType

	// bucketName is the name of the object storage bucket. It's populated when Type=ObjectStorageTypeGCS or ObjectStorageTypeS3.
	bucketName string

	// gcsServiceAccountKey is the GCS service account key in JSON format. It's populated when Type=ObjectStorageTypeGCS.
	gcsServiceAccountKey []byte

	// s3Credentials are the AWS credentials in JSON format. It's populated when Type=ObjectStorageTypeS3.
	s3Credentials []byte

	// s3AgentConfig is an optional custom Scylla Manager Agent configuration for S3-compatible storage (e.g., endpoint override for MinIO).
	// It's populated when Type=ObjectStorageTypeS3 and an agent config is provided.
	s3AgentConfig []byte
}

// NewGCSClusterObjectStorageSettings creates a new ClusterObjectStorageSettings for a cluster using a GCS bucket.
func NewGCSClusterObjectStorageSettings(bucketName string, serviceAccountKey []byte) (ClusterObjectStorageSettings, error) {
	if len(bucketName) == 0 {
		return ClusterObjectStorageSettings{}, errors.New("bucket name cannot be empty")
	}
	if len(serviceAccountKey) == 0 {
		return ClusterObjectStorageSettings{}, errors.New("gcs service account key cannot be empty")
	}

	return ClusterObjectStorageSettings{
		storageType:          ObjectStorageTypeGCS,
		bucketName:           bucketName,
		gcsServiceAccountKey: serviceAccountKey,
	}, nil
}

// NewS3ClusterObjectStorageSettings creates a new ClusterObjectStorageSettings for a cluster using an S3-compatible bucket.
// agentConfig is an optional custom Scylla Manager Agent configuration (e.g., for specifying a custom S3 endpoint like MinIO); pass nil when not needed.
func NewS3ClusterObjectStorageSettings(bucketName string, credentials, agentConfig []byte) (ClusterObjectStorageSettings, error) {
	if len(bucketName) == 0 {
		return ClusterObjectStorageSettings{}, errors.New("bucket name cannot be empty")
	}
	if len(credentials) == 0 {
		return ClusterObjectStorageSettings{}, errors.New("s3 credentials cannot be empty")
	}
	return ClusterObjectStorageSettings{
		storageType:   ObjectStorageTypeS3,
		bucketName:    bucketName,
		s3Credentials: credentials,
		s3AgentConfig: agentConfig,
	}, nil
}

// Type returns the type of object storage used by the cluster.
func (s ClusterObjectStorageSettings) Type() ObjectStorageType {
	return s.storageType
}

// BucketName returns the name of the object storage bucket.
func (s ClusterObjectStorageSettings) BucketName() string {
	return s.bucketName
}

// GCSServiceAccountKey returns the GCS service account key in JSON format.
// It's up to the caller to ensure that this is only called when Type is ObjectStorageTypeGCS.
func (s ClusterObjectStorageSettings) GCSServiceAccountKey() []byte {
	return s.gcsServiceAccountKey
}

// S3CredentialsFile returns the AWS credentials in JSON format.
// It's up to the caller to ensure that this is only called when Type is ObjectStorageTypeS3.
func (s ClusterObjectStorageSettings) S3CredentialsFile() []byte {
	return s.s3Credentials
}

// S3AgentConfig returns the optional custom Scylla Manager Agent configuration for S3-compatible storage.
// It's up to the caller to ensure that this is only called when Type is ObjectStorageTypeS3.
func (s ClusterObjectStorageSettings) S3AgentConfig() []byte {
	return s.s3AgentConfig
}
