package gapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

type LokiDerivedField struct {
	Name          string `json:"name"`
	MatcherRegex  string `json:"matcherRegex"`
	URL           string `json:"url"`
	DatasourceUID string `json:"datasourceUid,omitempty"`
}

// JSONData is a representation of the datasource `jsonData` property
type JSONData struct {
	// Used by all datasources
	TLSAuth                bool   `json:"tlsAuth,omitempty"`
	TLSAuthWithCACert      bool   `json:"tlsAuthWithCACert,omitempty"`
	TLSConfigurationMethod string `json:"tlsConfigurationMethod,omitempty"`
	TLSSkipVerify          bool   `json:"tlsSkipVerify,omitempty"`

	// Used by Athena
	Catalog        string `json:"catalog,omitempty"`
	Database       string `json:"database,omitempty"`
	OutputLocation string `json:"outputLocation,omitempty"`
	Workgroup      string `json:"workgroup,omitempty"`

	// Used by Github
	GitHubURL string `json:"githubUrl,omitempty"`

	// Used by Graphite
	GraphiteVersion string `json:"graphiteVersion,omitempty"`

	// Used by Prometheus, Elasticsearch, InfluxDB, MySQL, PostgreSQL and MSSQL
	TimeInterval string `json:"timeInterval,omitempty"`

	// Used by Elasticsearch
	// From Grafana 8.x esVersion is the semantic version of Elasticsearch.
	EsVersion                  string `json:"esVersion,omitempty"`
	TimeField                  string `json:"timeField,omitempty"`
	Interval                   string `json:"interval,omitempty"`
	LogMessageField            string `json:"logMessageField,omitempty"`
	LogLevelField              string `json:"logLevelField,omitempty"`
	MaxConcurrentShardRequests int64  `json:"maxConcurrentShardRequests,omitempty"`
	XpackEnabled               bool   `json:"xpack"`

	// Used by Cloudwatch
	CustomMetricsNamespaces string `json:"customMetricsNamespaces,omitempty"`
	TracingDatasourceUID    string `json:"tracingDatasourceUid,omitempty"`

	// Used by Cloudwatch, Athena
	AuthType      string `json:"authType,omitempty"`
	AssumeRoleArn string `json:"assumeRoleArn,omitempty"`
	DefaultRegion string `json:"defaultRegion,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	ExternalID    string `json:"externalId,omitempty"`
	Profile       string `json:"profile,omitempty"`

	// Used by Loki
	DerivedFields []LokiDerivedField `json:"derivedFields,omitempty"`
	MaxLines      int                `json:"maxLines,omitempty"`

	// Used by OpenTSDB
	TsdbVersion    int64 `json:"tsdbVersion,omitempty"`
	TsdbResolution int64 `json:"tsdbResolution,omitempty"`

	// Used by MSSQL
	Encrypt string `json:"encrypt,omitempty"`

	// Used by PostgreSQL
	Sslmode         string `json:"sslmode,omitempty"`
	PostgresVersion int64  `json:"postgresVersion,omitempty"`
	Timescaledb     bool   `json:"timescaledb"`

	// Used by MySQL, PostgreSQL and MSSQL
	MaxOpenConns    int64 `json:"maxOpenConns,omitempty"`
	MaxIdleConns    int64 `json:"maxIdleConns,omitempty"`
	ConnMaxLifetime int64 `json:"connMaxLifetime,omitempty"`

	// Used by Prometheus
	HTTPMethod   string `json:"httpMethod,omitempty"`
	QueryTimeout string `json:"queryTimeout,omitempty"`

	// Used by Stackdriver
	AuthenticationType string `json:"authenticationType,omitempty"`
	ClientEmail        string `json:"clientEmail,omitempty"`
	DefaultProject     string `json:"defaultProject,omitempty"`
	TokenURI           string `json:"tokenUri,omitempty"`

	// Used by Prometheus and Elasticsearch
	SigV4AssumeRoleArn string `json:"sigV4AssumeRoleArn,omitempty"`
	SigV4Auth          bool   `json:"sigV4Auth"`
	SigV4AuthType      string `json:"sigV4AuthType,omitempty"`
	SigV4ExternalID    string `json:"sigV4ExternalID,omitempty"`
	SigV4Profile       string `json:"sigV4Profile,omitempty"`
	SigV4Region        string `json:"sigV4Region,omitempty"`

	// Used by Prometheus and Loki
	ManageAlerts    bool   `json:"manageAlerts"`
	AlertmanagerUID string `json:"alertmanagerUid,omitempty"`

	// Used by Alertmanager
	Implementation string `json:"implementation,omitempty"`

	// Used by Sentry
	OrgSlug string `json:"orgSlug,omitempty"`
	URL     string `json:"url,omitempty"` // Sentry is not using the datasource URL attribute

	// Used by InfluxDB
	DefaultBucket string `json:"defaultBucket,omitempty"`
	Organization  string `json:"organization,omitempty"`
	Version       string `json:"version,omitempty"`

	// Used by Azure Monitor
	ClientID       string `json:"clientId,omitempty"`
	CloudName      string `json:"cloudName,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	TenantID       string `json:"tenantId,omitempty"`
}

// Marshal JSONData
func (d JSONData) Map() (map[string]interface{}, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]interface{})
	if err = json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}

	return fields, nil
}

// SecureJSONData is a representation of the datasource `secureJsonData` property
type SecureJSONData struct {
	// Used by all datasources
	TLSCACert         string `json:"tlsCACert,omitempty"`
	TLSClientCert     string `json:"tlsClientCert,omitempty"`
	TLSClientKey      string `json:"tlsClientKey,omitempty"`
	Password          string `json:"password,omitempty"`
	BasicAuthPassword string `json:"basicAuthPassword,omitempty"`

	// Used by Cloudwatch, Athena
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`

	// Used by Stackdriver
	PrivateKey string `json:"privateKey,omitempty"`

	// Used by Prometheus and Elasticsearch
	SigV4AccessKey string `json:"sigV4AccessKey,omitempty"`
	SigV4SecretKey string `json:"sigV4SecretKey,omitempty"`

	// Used by GitHub
	AccessToken string `json:"accessToken,omitempty"`

	// Used by Sentry
	AuthToken string `json:"authToken,omitempty"`

	// Used by Azure Monitor
	ClientSecret string `json:"clientSecret,omitempty"`
}

func (d SecureJSONData) Map() (map[string]interface{}, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]interface{})
	if err = json.Unmarshal(b, &fields); err != nil {
		return nil, err
	}

	return fields, nil
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{})
	for k, v := range m {
		clone[k] = v
	}
	return clone
}

func JSONDataWithHeaders(jsonData, secureJSONData map[string]interface{}, headers map[string]string) (map[string]interface{}, map[string]interface{}) {
	// Clone the maps so we don't modify the original
	jsonData = cloneMap(jsonData)
	secureJSONData = cloneMap(secureJSONData)

	idx := 1
	for name, value := range headers {
		jsonData[fmt.Sprintf("httpHeaderName%d", idx)] = name
		secureJSONData[fmt.Sprintf("httpHeaderValue%d", idx)] = value
		idx += 1
	}

	return jsonData, secureJSONData
}

func ExtractHeadersFromJSONData(jsonData, secureJSONData map[string]interface{}) (map[string]interface{}, map[string]interface{}, map[string]string) {
	// Clone the maps so we don't modify the original
	jsonData = cloneMap(jsonData)
	secureJSONData = cloneMap(secureJSONData)
	headers := make(map[string]string)

	for dataName, dataValue := range jsonData {
		if strings.HasPrefix(dataName, "httpHeaderName") {
			// Remove the header name from JSON data
			delete(jsonData, dataName)

			// Remove the header value from secure JSON data
			secureDataName := strings.Replace(dataName, "httpHeaderName", "httpHeaderValue", 1)
			delete(secureJSONData, secureDataName)

			headerName := dataValue.(string)
			headers[headerName] = "true" // We can't retrieve the headers, so we just set a dummy value
		}
	}

	return jsonData, secureJSONData, headers
}
