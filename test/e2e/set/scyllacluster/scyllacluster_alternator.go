// Copyright (C) 2024 ScyllaDB

package scyllacluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/smithy-go"
	"github.com/gocql/gocql"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	scyllav1 "github.com/scylladb/scylla-operator/pkg/api/scylla/v1"
	"github.com/scylladb/scylla-operator/pkg/controllerhelpers"
	oslices "github.com/scylladb/scylla-operator/pkg/helpers/slices"
	"github.com/scylladb/scylla-operator/pkg/naming"
	"github.com/scylladb/scylla-operator/pkg/pointer"
	scyllafixture "github.com/scylladb/scylla-operator/test/e2e/fixture/scylla"
	"github.com/scylladb/scylla-operator/test/e2e/framework"
	"github.com/scylladb/scylla-operator/test/e2e/utils"
	scyllaclusterverification "github.com/scylladb/scylla-operator/test/e2e/utils/verification/scyllacluster"
	"github.com/scylladb/scylla-operator/test/e2e/verification"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
)

const (
	cqlDefaultUser     = "cassandra"
	cqlDefaultPassword = "cassandra"
)

type Movie struct {
	Title string                 `dynamodbav:"title"`
	Year  int                    `dynamodbav:"year"`
	Info  map[string]interface{} `dynamodbav:"info"`
}

func (movie Movie) GetKey() map[string]types.AttributeValue {
	title, err := attributevalue.Marshal(movie.Title)
	o.Expect(err).NotTo(o.HaveOccurred())

	year, err := attributevalue.Marshal(movie.Year)
	o.Expect(err).NotTo(o.HaveOccurred())

	return map[string]types.AttributeValue{
		"title": title,
		"year":  year,
	}
}

var _ = g.Describe("ScyllaCluster", func() {
	f := framework.NewFramework("scyllacluster")

	g.It("should set up Alternator API when enabled", func() {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		cm, _, err := scyllafixture.ScyllaDBConfigTemplate.RenderObject(map[string]any{
			"config": strings.TrimPrefix(`
authenticator: PasswordAuthenticator
authorizer: CassandraAuthorizer
`, "\n"),
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Creating a ScyllaDB config ConfigMap enabling AuthN+AuthZ")
		cm, err = f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Create(ctx, cm, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		sc := f.GetDefaultScyllaCluster()
		o.Expect(sc.Spec.Datacenter.Racks).NotTo(o.BeEmpty())
		sc.Spec.Datacenter.Racks = sc.Spec.Datacenter.Racks[:1]
		sc.Spec.Datacenter.Racks[0].Members = 1
		sc.Spec.Datacenter.Racks[0].ScyllaConfig = cm.Name
		sc.Spec.Alternator = &scyllav1.AlternatorSpec{
			ServingCertificate: &scyllav1.TLSCertificate{
				Type: scyllav1.TLSCertificateTypeOperatorManaged,
				OperatorManagedOptions: &scyllav1.OperatorManagedTLSCertificateOptions{
					AdditionalIPAddresses: []string{"42.42.42.42"},
					AdditionalDNSNames:    []string{"scylla.operator.rocks"},
				},
			},
		}

		framework.By("Creating a ScyllaCluster with 1 member")
		sc, err = f.ScyllaClient().ScyllaV1().ScyllaClusters(f.Namespace()).Create(ctx, sc, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Waiting for the ScyllaCluster to roll out (RV=%s)", sc.ResourceVersion)
		waitCtx1, waitCtx1Cancel := utils.ContextForRollout(ctx, sc)
		defer waitCtx1Cancel()
		sc, err = controllerhelpers.WaitForScyllaClusterState(waitCtx1, f.ScyllaClient().ScyllaV1().ScyllaClusters(sc.Namespace), sc.Name, controllerhelpers.WaitForStateOptions{}, utils.IsScyllaClusterRolledOut)
		o.Expect(err).NotTo(o.HaveOccurred())

		scyllaclusterverification.Verify(ctx, f.KubeClient(), f.ScyllaClient(), sc)

		svcIP, err := utils.GetIdentityServiceIP(ctx, f.KubeClient().CoreV1(), sc)
		o.Expect(err).NotTo(o.HaveOccurred())

		framework.By("Fetching Alternator token using CQL")

		cqlConfig := gocql.NewCluster(svcIP)
		cqlConfig.Authenticator = gocql.PasswordAuthenticator{
			Username: cqlDefaultUser,
			Password: cqlDefaultPassword,
		}
		cqlConfig.Consistency = gocql.All
		cqlSession, err := cqlConfig.CreateSession()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer cqlSession.Close()

		awsCredentials := aws.Credentials{
			AccessKeyID:     cqlDefaultUser,
			SecretAccessKey: "",
		}

		q := cqlSession.Query(
			`SELECT salted_hash FROM system.roles WHERE role = ?`,
			awsCredentials.AccessKeyID,
		).WithContext(ctx)
		err = q.Scan(&awsCredentials.SecretAccessKey)
		o.Expect(err).NotTo(o.HaveOccurred())
		q.Release()
		o.Expect(awsCredentials.SecretAccessKey).NotTo(o.BeEmpty())

		wrongAWSCredentials := awsCredentials
		wrongAWSCredentials.SecretAccessKey += "-wrong"

		framework.By("Verifying that Alternator is configured correctly")

		alternatorServingCertsSecret, err := f.KubeClient().CoreV1().Secrets(f.Namespace()).Get(ctx, fmt.Sprintf("%s-alternator-local-serving-certs", sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		alternatorServingCerts, _, _, _ := verification.VerifyAndParseTLSCert(alternatorServingCertsSecret, verification.TLSCertOptions{
			IsCA:     pointer.Ptr(false),
			KeyUsage: pointer.Ptr(x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature),
		})
		o.Expect(alternatorServingCerts).To(o.HaveLen(1))
		alternatorServingCert := alternatorServingCerts[0]
		o.Expect(alternatorServingCert.DNSNames).To(o.ConsistOf(
			fmt.Sprintf("%s-client.%s.svc", sc.Name, sc.Namespace),
			fmt.Sprintf("%s.%s.svc", naming.MemberServiceNameForScyllaCluster(sc.Spec.Datacenter.Racks[0], sc, 0), sc.Namespace),
			"scylla.operator.rocks",
		))
		alternatorServingCertIPStrings := oslices.ConvertSlice[string](
			alternatorServingCert.IPAddresses,
			func(from net.IP) string {
				return from.String()
			},
		)
		o.Expect(alternatorServingCertIPStrings).To(o.ContainElements(
			svcIP,
			"42.42.42.42",
		))

		alternatorServingCABundleConfigMap, err := f.KubeClient().CoreV1().ConfigMaps(f.Namespace()).Get(ctx, fmt.Sprintf("%s-alternator-local-serving-ca", sc.Name), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		alternatorServingCACerts, _ := verification.VerifyAndParseCABundle(alternatorServingCABundleConfigMap)
		o.Expect(alternatorServingCACerts).To(o.HaveLen(1))
		alternatorServingCACert := alternatorServingCACerts[0]
		alternatorServingCAPool := x509.NewCertPool()
		alternatorServingCAPool.AddCert(alternatorServingCACert)

		for _, tc := range []struct {
			url        string
			serverName string
		}{
			{
				url:        fmt.Sprintf("https://%s:8043", svcIP),
				serverName: "",
			},
			{
				url:        fmt.Sprintf("https://%s:8043", svcIP),
				serverName: fmt.Sprintf("%s-client.%s.svc", sc.Name, sc.Namespace),
			},
		} {

			httpClient := awshttp.NewBuildableClient().WithTransportOptions(func(transport *http.Transport) {
				transport.TLSClientConfig = &tls.Config{
					ServerName: tc.serverName,
					RootCAs:    alternatorServingCAPool,
				}
			})
			config := aws.Config{
				HTTPClient: httpClient,
				Credentials: credentials.StaticCredentialsProvider{
					Value: awsCredentials,
				},
				RetryMaxAttempts: 0,
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			tableName := names.SimpleNameGenerator.GenerateName("scylla-operator-e2e-")
			framework.By("Testing Alternator API on URL %q and server name %q", tc.url, tc.serverName)

			applyOpts := func(opts *dynamodb.Options) {
				opts.BaseEndpoint = pointer.Ptr(tc.url)
			}
			dynamodbClient := dynamodb.NewFromConfig(config, applyOpts)
			o.Expect(dynamodbClient).NotTo(o.BeNil())

			_, err = dynamodbClient.CreateTable(ctx, &dynamodb.CreateTableInput{
				TableName: &tableName,
				AttributeDefinitions: []types.AttributeDefinition{
					{
						AttributeName: pointer.Ptr("year"),
						AttributeType: types.ScalarAttributeTypeN,
					},
					{
						AttributeName: pointer.Ptr("title"),
						AttributeType: types.ScalarAttributeTypeS,
					},
				},
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: pointer.Ptr("year"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: pointer.Ptr("title"),
						KeyType:       types.KeyTypeRange,
					},
				},
				ProvisionedThroughput: &types.ProvisionedThroughput{
					ReadCapacityUnits:  pointer.Ptr(int64(42)),
					WriteCapacityUnits: pointer.Ptr(int64(42)),
				},
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			waiter := dynamodb.NewTableExistsWaiter(dynamodbClient)
			err = waiter.Wait(ctx, &dynamodb.DescribeTableInput{TableName: &tableName}, 1*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			movie := &Movie{
				Title: "title",
				Year:  1970,
				Info: map[string]interface{}{
					"foo": "bar",
				},
			}
			itemAttr, err := attributevalue.MarshalMap(movie)
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = dynamodbClient.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: &tableName,
				Item:      itemAttr,
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			gio, err := dynamodbClient.GetItem(ctx, &dynamodb.GetItemInput{
				TableName: &tableName,
				Key:       movie.GetKey(),
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(gio.Item).NotTo(o.BeNil())

			gotMovie := &Movie{}
			err = attributevalue.UnmarshalMap(gio.Item, gotMovie)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(gotMovie).To(o.BeEquivalentTo(movie))

			framework.By("Making sure Alternator API on URL %q with Server name %q refuses unauthorized calls", tc.url, tc.serverName)

			unauthorizedDynamoDBClient := dynamodb.NewFromConfig(config, func(opts *dynamodb.Options) {
				applyOpts(opts)
				opts.Credentials = credentials.StaticCredentialsProvider{
					Value: wrongAWSCredentials,
				}
			})
			o.Expect(unauthorizedDynamoDBClient).NotTo(o.BeNil())

			_, err = unauthorizedDynamoDBClient.GetItem(ctx, &dynamodb.GetItemInput{
				TableName: &tableName,
				Key:       movie.GetKey(),
			})
			o.Expect(err).To(o.MatchError(&smithy.GenericAPIError{
				Code:    "UnrecognizedClientException",
				Message: "wrong signature",
			}))

			framework.By("Cleaning up table %q", tableName)

			_, err = dynamodbClient.DeleteTable(ctx, &dynamodb.DeleteTableInput{
				TableName: &tableName,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
