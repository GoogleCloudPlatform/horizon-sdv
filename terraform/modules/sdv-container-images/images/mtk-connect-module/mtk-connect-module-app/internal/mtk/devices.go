// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mtk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Device is one device of an agent as returned by the REST API. Index is 0 when the server did not say.
type Device struct {
	ID    string
	Index int
	Name  string
}

type rawDevice struct {
	ID    flexString `json:"id"`
	Index flexString `json:"index"`
	Name  string     `json:"name"`
}

func (r rawDevice) device() Device {
	idx, _ := strconv.Atoi(string(r.Index))
	return Device{ID: string(r.ID), Index: idx, Name: r.Name}
}

func devicePath(agentID string, index int) string {
	return "/agents/" + url.PathEscape(agentID) + "/devices/" + strconv.Itoa(index)
}

// FindDevice returns the device with the given index of the agent with the given registration, or nil.
// It sends the same query as create-testbench.js.
func (c *Client) FindDevice(ctx context.Context, registration string, index int) (*Device, error) {
	b, err := json.Marshal(map[string]interface{}{"agent.registration": registration, "index": index})
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []rawDevice `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/devices?q="+url.QueryEscape(string(b)), nil, &out); err != nil {
		return nil, err
	}
	switch len(out.Data) {
	case 0:
		return nil, nil
	case 1:
		d := out.Data[0].device()
		if d.Index == 0 {
			d.Index = index
		}
		return &d, nil
	default:
		return nil, fmt.Errorf("mtk-connect: %d devices share index %d of agent %q", len(out.Data), index, registration)
	}
}

// CreateDevice adds a device to the agent. The server numbers devices itself, in creation order.
func (c *Client) CreateDevice(ctx context.Context, agentID, name string) (Device, error) {
	var out struct {
		Data rawDevice `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/agents/"+url.PathEscape(agentID)+"/devices", map[string]string{"name": name}, &out); err != nil {
		return Device{}, err
	}
	d := out.Data.device()
	if d.Name == "" {
		d.Name = name
	}
	return d, nil
}

// ConfigureDevice sets the interface of a device. payload is the body built by the profiles package.
func (c *Client) ConfigureDevice(ctx context.Context, agentID string, index int, payload map[string]interface{}) error {
	return c.do(ctx, http.MethodPatch, devicePath(agentID, index), payload, nil)
}

// DeleteDevice removes a device. A device or agent that is already gone is not an error.
func (c *Client) DeleteDevice(ctx context.Context, agentID string, index int) error {
	err := c.do(ctx, http.MethodDelete, devicePath(agentID, index), nil, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}
