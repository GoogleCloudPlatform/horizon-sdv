// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wsapi

import (
	"context"
	"fmt"
	"sync"

	iampb "cloud.google.com/go/iam/apiv1/iampb"
	workstations "cloud.google.com/go/workstations/apiv1"
	pb "cloud.google.com/go/workstations/apiv1/workstationspb"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// API is the Cloud Workstations surface used by the controller.
// It has no start or stop method. Power stays outside this client.
type API interface {
	GetConfig(ctx context.Context, name string) (*pb.WorkstationConfig, error)
	PatchConfigPosture(ctx context.Context, name, etag string, posture Posture) error
	GetIamPolicy(ctx context.Context, resource string) (*iampb.Policy, error)
	SetIamPolicy(ctx context.Context, resource string, policy *iampb.Policy) (*iampb.Policy, error)
}

// Client calls the Cloud Workstations API with application default credentials.
type Client struct {
	raw *workstations.Client
}

// New builds a client. Application default credentials are resolved on the first RPC,
// so process startup does not require a credential file.
func New(ctx context.Context) (*Client, error) {
	raw, err := workstations.NewClient(ctx, option.WithTokenSource(&lazyADC{}))
	if err != nil {
		return nil, fmt.Errorf("workstations client: %w", err)
	}
	return &Client{raw: raw}, nil
}

// lazyADC looks up application default credentials the first time a token is requested.
type lazyADC struct {
	once sync.Once
	src  oauth2.TokenSource
	err  error
}

func (l *lazyADC) Token() (*oauth2.Token, error) {
	l.once.Do(func() {
		l.src, l.err = google.DefaultTokenSource(context.Background(), workstations.DefaultAuthScopes()...)
	})
	if l.err != nil {
		return nil, l.err
	}
	return l.src.Token()
}

// Close releases the underlying connection.
func (c *Client) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Close()
}

// GetConfig reads one workstation configuration.
func (c *Client) GetConfig(ctx context.Context, name string) (*pb.WorkstationConfig, error) {
	return c.raw.GetWorkstationConfig(ctx, &pb.GetWorkstationConfigRequest{Name: name})
}

// PatchConfigPosture sends a posture-only config update and does not wait for the operation.
// The caller reads the config back on a later reconcile.
func (c *Client) PatchConfigPosture(ctx context.Context, name, etag string, posture Posture) error {
	req, err := BuildPostureUpdate(name, etag, posture)
	if err != nil {
		return err
	}
	_, err = c.raw.UpdateWorkstationConfig(ctx, req)
	return err
}

// GetIamPolicy reads the policy at version 3 so conditional bindings are preserved.
func (c *Client) GetIamPolicy(ctx context.Context, resource string) (*iampb.Policy, error) {
	return c.raw.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: resource,
		Options:  &iampb.GetPolicyOptions{RequestedPolicyVersion: 3},
	})
}

// SetIamPolicy writes the policy, including the version and etag from GetIamPolicy.
func (c *Client) SetIamPolicy(ctx context.Context, resource string, policy *iampb.Policy) (*iampb.Policy, error) {
	return c.raw.SetIamPolicy(ctx, &iampb.SetIamPolicyRequest{
		Resource: resource,
		Policy:   policy,
	})
}
