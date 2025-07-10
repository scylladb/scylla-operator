package tests

import (
	"fmt"
	"os"

	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/spf13/cobra"
	apimachineryutilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// ObjectStorageOptions holds the options for object storage configuration in tests.
type ObjectStorageOptions struct {
	ClusterObjectStorageSettings       *framework.ClusterObjectStorageSettings
	WorkerClusterObjectStorageSettings map[string]framework.ClusterObjectStorageSettings

	// Raw CLI flags. Do not use outside of this struct methods.
	objectStorageBucket             string
	gcsServiceAccountKeyPath        string
	s3CredentialsFilePath           string
	workerObjectStorageBuckets      map[string]string
	workerGCSServiceAccountKeyPaths map[string]string
	workerS3CredentialsFilePaths    map[string]string
}

func NewObjectStorageOptions() ObjectStorageOptions {
	return ObjectStorageOptions{
		WorkerClusterObjectStorageSettings: make(map[string]framework.ClusterObjectStorageSettings),

		workerObjectStorageBuckets:      make(map[string]string),
		workerGCSServiceAccountKeyPaths: make(map[string]string),
		workerS3CredentialsFilePaths:    make(map[string]string),
	}
}

func (oso *ObjectStorageOptions) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&oso.objectStorageBucket, "object-storage-bucket", "", oso.objectStorageBucket, "Name of the object storage bucket.")
	cmd.PersistentFlags().StringVarP(&oso.gcsServiceAccountKeyPath, "gcs-service-account-key-path", "", oso.gcsServiceAccountKeyPath, "Path to a file containing a GCS service account key.")
	cmd.PersistentFlags().StringVarP(&oso.s3CredentialsFilePath, "s3-credentials-file-path", "", oso.s3CredentialsFilePath, "Path to the AWS credentials file providing access to the S3 bucket.")

	cmd.PersistentFlags().StringToStringVarP(&oso.workerGCSServiceAccountKeyPaths, "worker-gcs-service-account-key-paths", "", oso.workerGCSServiceAccountKeyPaths, "Map of worker cluster identifiers to GCS service account key paths. Used in multi-datacenter setups.")
	cmd.PersistentFlags().StringToStringVarP(&oso.workerS3CredentialsFilePaths, "worker-s3-credentials-file-paths", "", oso.workerS3CredentialsFilePaths, "Map of worker cluster identifiers to S3 credentials file paths. Used in multi-datacenter setups.")
	cmd.PersistentFlags().StringToStringVarP(&oso.workerObjectStorageBuckets, "worker-object-storage-buckets", "", oso.workerObjectStorageBuckets, "Map of worker cluster identifier to object storage bucket names. Used in multi-datacenter setups.")
}

func (oso *ObjectStorageOptions) Validate() error {
	var errors []error

	if len(oso.gcsServiceAccountKeyPath) > 0 && len(oso.objectStorageBucket) == 0 {
		errors = append(errors, fmt.Errorf("object-storage-bucket can't be empty when gcs-service-account-key-path is provided"))
	}
	if len(oso.s3CredentialsFilePath) > 0 && len(oso.objectStorageBucket) == 0 {
		errors = append(errors, fmt.Errorf("object-storage-bucket can't be empty when s3-credentials-file-path is provided"))
	}
	if len(oso.objectStorageBucket) > 0 && len(oso.gcsServiceAccountKeyPath) == 0 && len(oso.s3CredentialsFilePath) == 0 {
		errors = append(errors, fmt.Errorf("either gcs-service-account-key-path or s3-credentials-file-path must be set when object-storage-bucket is provided"))
	}
	if len(oso.gcsServiceAccountKeyPath) > 0 && len(oso.s3CredentialsFilePath) > 0 {
		errors = append(errors, fmt.Errorf("gcs-service-account-key-path and s3-credentials-file-path can't be set simultaneously"))
	}

	if len(oso.workerGCSServiceAccountKeyPaths) > 0 || len(oso.workerS3CredentialsFilePaths) > 0 || len(oso.workerObjectStorageBuckets) > 0 {
		if len(oso.gcsServiceAccountKeyPath) > 0 || len(oso.s3CredentialsFilePath) > 0 || len(oso.objectStorageBucket) > 0 {
			errors = append(errors, fmt.Errorf("worker-* flags cannot be used with single bucket flags"))
		}
		if len(oso.workerGCSServiceAccountKeyPaths) > 0 && len(oso.workerS3CredentialsFilePaths) > 0 {
			errors = append(errors, fmt.Errorf("only one of worker-gcs-service-account-key-paths or worker-s3-credentials-file-paths can be set"))
		}
		if len(oso.workerObjectStorageBuckets) != 0 && len(oso.workerGCSServiceAccountKeyPaths) == 0 && len(oso.workerS3CredentialsFilePaths) == 0 {
			errors = append(errors, fmt.Errorf("either worker-gcs-service-account-key-paths or worker-s3-credentials-file-paths must be set when worker-object-storage-buckets is provided"))
		}

		if len(oso.workerGCSServiceAccountKeyPaths) > 0 && !equalKeys(oso.workerObjectStorageBuckets, oso.workerGCSServiceAccountKeyPaths) {
			errors = append(errors, fmt.Errorf("worker-object-storage-buckets must have the same keys as worker-gcs-service-account-key-paths"))
		}
		if len(oso.workerS3CredentialsFilePaths) > 0 && !equalKeys(oso.workerObjectStorageBuckets, oso.workerS3CredentialsFilePaths) {
			errors = append(errors, fmt.Errorf("worker-object-storage-buckets must have the same keys as worker-s3-credentials-file-paths"))
		}
	}

	return apimachineryutilerrors.NewAggregate(errors)
}

func (oso *ObjectStorageOptions) Complete() error {
	if len(oso.gcsServiceAccountKeyPath) > 0 {
		gcsServiceAccountKey, err := os.ReadFile(oso.gcsServiceAccountKeyPath)
		if err != nil {
			return fmt.Errorf("can't read gcs service account key file %q: %w", oso.gcsServiceAccountKeyPath, err)
		}

		s, err := framework.NewGCSClusterObjectStorageSettings(oso.objectStorageBucket, gcsServiceAccountKey)
		if err != nil {
			return fmt.Errorf("can't create GCS cluster object storage settings: %w", err)
		}
		oso.ClusterObjectStorageSettings = &s
	}

	if len(oso.s3CredentialsFilePath) > 0 {
		s3CredentialsFile, err := os.ReadFile(oso.s3CredentialsFilePath)
		if err != nil {
			return fmt.Errorf("can't read s3 credentials file %q: %w", oso.s3CredentialsFilePath, err)
		}

		s, err := framework.NewS3ClusterObjectStorageSettings(oso.objectStorageBucket, s3CredentialsFile)
		if err != nil {
			return fmt.Errorf("can't create S3 cluster object storage settings: %w", err)
		}
		oso.ClusterObjectStorageSettings = &s
	}

	for worker, path := range oso.workerS3CredentialsFilePaths {
		credentials, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("can't read S3 credentials file for worker %q at %q: %w", worker, path, err)
		}
		s, err := framework.NewS3ClusterObjectStorageSettings(oso.workerObjectStorageBuckets[worker], credentials)
		if err != nil {
			return fmt.Errorf("can't create S3 cluster object storage settings for worker %q: %w", worker, err)
		}
		oso.WorkerClusterObjectStorageSettings[worker] = s

	}

	for worker, path := range oso.workerGCSServiceAccountKeyPaths {
		credentials, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("can't read GCS service account key file for worker %q at %q: %w", worker, path, err)
		}
		s, err := framework.NewGCSClusterObjectStorageSettings(oso.workerObjectStorageBuckets[worker], credentials)
		if err != nil {
			return fmt.Errorf("can't create GCS cluster object storage settings for worker %q: %w", worker, err)
		}
		oso.WorkerClusterObjectStorageSettings[worker] = s
	}

	return nil
}

func equalKeys(m1, m2 map[string]string) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k := range m1 {
		if _, ok := m2[k]; !ok {
			return false
		}
	}
	return true
}
