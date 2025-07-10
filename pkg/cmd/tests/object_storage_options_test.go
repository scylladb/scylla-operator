package tests

import (
	"strings"
	"testing"
)

func TestObjectStorageOptionsValidation(t *testing.T) {
	t.Parallel()

	tt := []struct {
		name                  string
		options               ObjectStorageOptions
		expectedErrorContains string
	}{
		{
			name: "valid single-DC options with s3",
			options: ObjectStorageOptions{
				s3CredentialsFilePath: "/path/to/s3/credentials",
				objectStorageBucket:   "test-bucket",
			},
		},
		{
			name: "valid single-DC options with GCS",
			options: ObjectStorageOptions{
				gcsServiceAccountKeyPath: "/path/to/gcs/key.json",
				objectStorageBucket:      "test-bucket",
			},
		},
		{
			name: "valid multi-DC options with s3",
			options: ObjectStorageOptions{
				workerObjectStorageBuckets: map[string]string{
					"dc1": "bucket-dc1",
					"dc2": "bucket-dc2",
				},
				workerS3CredentialsFilePaths: map[string]string{
					"dc1": "/path/to/s3/credentials-dc1",
					"dc2": "/path/to/s3/credentials-dc2",
				},
			},
		},
		{
			name: "valid multi-DC options with GCS",
			options: ObjectStorageOptions{
				workerObjectStorageBuckets: map[string]string{
					"dc1": "bucket-dc1",
					"dc2": "bucket-dc2",
				},
				workerGCSServiceAccountKeyPaths: map[string]string{
					"dc1": "/path/to/gcs/key-dc1.json",
					"dc2": "/path/to/gcs/key-dc2.json",
				},
			},
		},
		{
			name: "invalid single-DC options with GCS without bucket",
			options: ObjectStorageOptions{
				gcsServiceAccountKeyPath: "/path/to/gcs/key.json",
			},
			expectedErrorContains: "object-storage-bucket can't be empty when gcs-service-account-key-path is provided",
		},
		{
			name: "invalid single-DC options with S3 without bucket",
			options: ObjectStorageOptions{
				s3CredentialsFilePath: "/path/to/s3/credentials",
			},
			expectedErrorContains: "object-storage-bucket can't be empty when s3-credentials-file-path is provided",
		},
		{
			name: "invalid single-DC options with bucket but no credentials",
			options: ObjectStorageOptions{
				objectStorageBucket: "test-bucket",
			},
			expectedErrorContains: "either gcs-service-account-key-path or s3-credentials-file-path must be set when object-storage-bucket is provided",
		},
		{
			name: "invalid single-DC options with both GCS and S3 credentials",
			options: ObjectStorageOptions{
				gcsServiceAccountKeyPath: "/path/to/gcs/key.json",
				s3CredentialsFilePath:    "/path/to/s3/credentials",
				objectStorageBucket:      "test-bucket",
			},
			expectedErrorContains: "gcs-service-account-key-path and s3-credentials-file-path can't be set simultaneously",
		},
		{
			name: "invalid multi-DC options with GCS without bucket",
			options: ObjectStorageOptions{
				workerGCSServiceAccountKeyPaths: map[string]string{
					"dc1": "/path/to/gcs/key-dc1.json",
					"dc2": "/path/to/gcs/key-dc2.json",
				},
			},
			expectedErrorContains: "worker-object-storage-buckets must have the same keys as worker-gcs-service-account-key-paths",
		},
		{
			name: "invalid multi-DC options with S3 without bucket",
			options: ObjectStorageOptions{
				workerS3CredentialsFilePaths: map[string]string{
					"dc1": "/path/to/s3/credentials-dc1",
					"dc2": "/path/to/s3/credentials-dc2",
				},
			},
			expectedErrorContains: "worker-object-storage-buckets must have the same keys as worker-s3-credentials-file-paths",
		},
		{
			name: "invalid multi-DC options with GCS and S3 credentials",
			options: ObjectStorageOptions{
				workerObjectStorageBuckets: map[string]string{
					"dc1": "bucket-dc1",
					"dc2": "bucket-dc2",
				},
				workerGCSServiceAccountKeyPaths: map[string]string{
					"dc1": "/path/to/gcs/key-dc1.json",
					"dc2": "/path/to/gcs/key-dc2.json",
				},
				workerS3CredentialsFilePaths: map[string]string{
					"dc1": "/path/to/s3/credentials-dc1",
					"dc2": "/path/to/s3/credentials-dc2",
				},
			},
			expectedErrorContains: "only one of worker-gcs-service-account-key-paths or worker-s3-credentials-file-paths can be set",
		},
		{
			name: "invalid multi-DC options with bucket but no credentials",
			options: ObjectStorageOptions{
				workerObjectStorageBuckets: map[string]string{
					"dc1": "bucket-dc1",
					"dc2": "bucket-dc2",
				},
			},
			expectedErrorContains: "either worker-gcs-service-account-key-paths or worker-s3-credentials-file-paths must be set when worker-object-storage-buckets is provided",
		},
		{
			name: "invalid multi-DC S3 options with mismatched keys",
			options: ObjectStorageOptions{
				workerObjectStorageBuckets: map[string]string{
					"dc1": "bucket-dc1",
					"dc2": "bucket-dc2",
				},
				workerS3CredentialsFilePaths: map[string]string{
					"dc1": "/path/to/s3/credentials-dc1",
					// Missing "dc2" key
				},
			},
			expectedErrorContains: "worker-object-storage-buckets must have the same keys as worker-s3-credentials-file-paths",
		},
		{
			name: "invalid multi-DC GCS options with mismatched keys",
			options: ObjectStorageOptions{
				workerObjectStorageBuckets: map[string]string{
					"dc1": "bucket-dc1",
					"dc2": "bucket-dc2",
				},
				workerGCSServiceAccountKeyPaths: map[string]string{
					"dc1": "/path/to/gcs/key-dc1.json",
					// Missing "dc2" key
				},
			},
			expectedErrorContains: "worker-object-storage-buckets must have the same keys as worker-gcs-service-account-key-paths",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.options.Validate()
			if err != nil {
				if tc.expectedErrorContains == "" {
					t.Errorf("expected no error, got %v", err)
				} else if !strings.Contains(err.Error(), tc.expectedErrorContains) {
					t.Errorf("expected error to contain %q, got %q", tc.expectedErrorContains, err.Error())
				}
			} else if tc.expectedErrorContains != "" {
				t.Errorf("expected error containing %q, got nil", tc.expectedErrorContains)
			}
		})
	}
}
