// Copyright 2018 The Jetstack Navigator Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package client implements requests to JMX MBeans using the Jolokia
// HTTP<->Bridge.
//
// For more information on the Jolokia protocol, visit:
// https://jolokia.org/reference/html/protocol.html

package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/pborman/uuid"

	"bytes"
	"github.com/yanniszark/go-nodetool/version"
)

const (
	JolokiaRequestTypeRead = "read"
	JolokiaRequestTypeExec = "exec"
)

type StorageService struct {
	HostIdMap        map[string]uuid.UUID `json:""`
	LiveNodes        []string             `json:""`
	UnreachableNodes []string             `json:""`
	LeavingNodes     []string             `json:""`
	JoiningNodes     []string             `json:""`
	MovingNodes      []string             `json:""`
	LocalHostId      uuid.UUID            `json:""`
	ReleaseVersion   *version.Version     `json:""`
	OperationMode    string               `json:""`
}

type Interface interface {
	Do(interface{}) ([]byte, error)
}

type client struct {
	baseURL *url.URL
	client  *http.Client
}

var _ Interface = &client{}

func New(baseURL *url.URL, c *http.Client) Interface {
	return &client{
		baseURL: baseURL,
		client:  c,
	}
}

type JolokiaExecRequest struct {
	Type      string   `json:"type"`
	MBean     string   `json:"mbean"`
	Operation string   `json:"operation"`
	Arguments []string `json:"arguments"`
}

type JolokiaReadRequest struct {
	Type      string   `json:"type"`
	MBean     string   `json:"mbean"`
	Attribute []string `json:"attribute"`
}

type JolokiaResponse struct {
	Value interface{} `json:"value"`
}

func (c *client) Do(request interface{}) ([]byte, error) {

	// Check if request is a known request struct
	switch request.(type) {
	case *JolokiaReadRequest:
		r := request.(*JolokiaReadRequest)
		r.Type = JolokiaRequestTypeRead
	case *JolokiaExecRequest:
		r := request.(*JolokiaExecRequest)
		r.Type = JolokiaRequestTypeExec
	default:
		return nil, fmt.Errorf("unknown request type %T", request)
	}

	reqJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request into json: %v", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL.String(), bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate HTTP request: %v", err)
	}
	httpReq.Header.Set("User-Agent", "go-nodetool")

	response, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send HTTP request: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"unexpected server response code for request %s. "+
				"Expected %d. Got %d.",
			httpReq.URL,
			http.StatusOK,
			response.StatusCode,
		)
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	out := &JolokiaResponse{}

	err = json.Unmarshal(body, out)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON response: %v", err)
	}
	valueBytes := []byte{}
	if out.Value != nil {
		valueBytes, err = json.Marshal(out.Value)
	}

	if err != nil {
		return nil, err
	}
	return valueBytes, nil
}
