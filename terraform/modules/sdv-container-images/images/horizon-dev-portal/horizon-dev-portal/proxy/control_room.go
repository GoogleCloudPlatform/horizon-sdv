// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	controlRoomCacheTTL         = 5 * time.Second
	controlRoomMinFreshInterval = 2 * time.Second
	controlRoomUpstreamTimeout  = 8 * time.Second
	gcpMetadataTokenURL         = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
	gcpMetadataProjectIDURL     = "http://metadata.google.internal/computeMetadata/v1/project/project-id"
	defaultGCPProjectID         = "qwiklabs-asl-00-8dfd084a9673"
	defaultGCPRegion            = "europe-west3"
	defaultArtifactRepo         = "horizon-sdv"
	defaultJenkinsExporterURL   = "http://jenkins-builds-exporter.jenkins.svc.cluster.local:8085/builds"
)

type ControlRoomResponse struct {
	GeneratedAt       string                   `json:"generatedAt"`
	ProjectID         string                   `json:"projectId"`
	PrimaryRegion     string                   `json:"primaryRegion"`
	Workstations      ControlRoomWorkstations  `json:"workstations"`
	Compute           ControlRoomCompute       `json:"compute"`
	ArtifactImages    []ContainerImageSummary  `json:"artifactImages"`
	BuildBundles      []BuildArtifactBundle    `json:"buildBundles"`
	ArchivedWorkflows []ArchivedWorkflowItem   `json:"archivedWorkflows"`
	JenkinsBuilds     []JenkinsBuildItem       `json:"jenkinsBuilds"`
	ABFS              ControlRoomABFS          `json:"abfs"`
	Warnings          []string                 `json:"warnings,omitempty"`
}

type ControlRoomWorkstations struct {
	Clusters     []WorkstationClusterItem  `json:"clusters"`
	Configs      []WorkstationConfigItem   `json:"configs"`
	Workstations []WorkstationInstanceItem `json:"workstations"`
}

type WorkstationClusterItem struct {
	Name           string `json:"name"`
	Region         string `json:"region"`
	ControlPlaneIP string `json:"controlPlaneIp,omitempty"`
	Subnetwork     string `json:"subnetwork,omitempty"`
	CreateTime     string `json:"createTime,omitempty"`
}

type WorkstationConfigItem struct {
	Name                 string   `json:"name"`
	Region               string   `json:"region"`
	Cluster              string   `json:"cluster"`
	MachineType          string   `json:"machineType"`
	GPUType              string   `json:"gpuType,omitempty"`
	GPUCount             int      `json:"gpuCount,omitempty"`
	BootDiskSizeGb       int      `json:"bootDiskSizeGb,omitempty"`
	PersistentDiskSizeGb int      `json:"persistentDiskSizeGb,omitempty"`
	PersistentDiskType   string   `json:"persistentDiskType,omitempty"`
	SourceSnapshot       string   `json:"sourceSnapshot,omitempty"`
	ContainerImage       string   `json:"containerImage"`
	IdleTimeout          string   `json:"idleTimeout"`
	RunningTimeout       string   `json:"runningTimeout"`
	ReplicaZones         []string `json:"replicaZones,omitempty"`
	Degraded             bool     `json:"degraded,omitempty"`
	CreateTime           string   `json:"createTime,omitempty"`
}

type WorkstationInstanceItem struct {
	Name                 string `json:"name"`
	DisplayName          string `json:"displayName,omitempty"`
	Region               string `json:"region"`
	Cluster              string `json:"cluster"`
	Config               string `json:"config"`
	State                string `json:"state"`
	Host                 string `json:"host,omitempty"`
	LaunchURL            string `json:"launchUrl,omitempty"`
	RuntimeHostVM        string `json:"runtimeHostVm,omitempty"`
	RuntimeZone          string `json:"runtimeZone,omitempty"`
	MachineType          string `json:"machineType,omitempty"`
	GPUType              string `json:"gpuType,omitempty"`
	ContainerImage       string `json:"containerImage,omitempty"`
	SourceSnapshot       string `json:"sourceSnapshot,omitempty"`
	PersistentDiskSizeGb int    `json:"persistentDiskSizeGb,omitempty"`
	CreateTime           string `json:"createTime,omitempty"`
	StartTime            string `json:"startTime,omitempty"`
	UpdateTime           string `json:"updateTime,omitempty"`
}

type ControlRoomCompute struct {
	VirtualDevices    []ComputeInstanceItem `json:"virtualDevices"`
	WorkstationVMs    []ComputeInstanceItem `json:"workstationVMs"`
	BuildInfraVMs     []ComputeInstanceItem `json:"buildInfraVMs"`
	ClusterNodes      []ComputeInstanceItem `json:"clusterNodes"`
	Snapshots         []DiskSnapshotItem    `json:"snapshots"`
	InstanceTemplates []InstanceTemplateItem `json:"instanceTemplates"`
}

type ComputeInstanceItem struct {
	Name              string            `json:"name"`
	Role              string            `json:"role"`
	Status            string            `json:"status"`
	Zone              string            `json:"zone"`
	Region            string            `json:"region"`
	MachineType       string            `json:"machineType"`
	Architecture      string            `json:"architecture"`
	DiskSizeGb        int64             `json:"diskSizeGb,omitempty"`
	InternalIP        string            `json:"internalIp,omitempty"`
	GPUType           string            `json:"gpuType,omitempty"`
	GPUCount          int               `json:"gpuCount,omitempty"`
	CreationTimestamp string            `json:"creationTimestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

type DiskSnapshotItem struct {
	Name              string            `json:"name"`
	Status            string            `json:"status"`
	DiskSizeGb        int64             `json:"diskSizeGb"`
	StorageBytes      int64             `json:"storageBytes"`
	CreationTimestamp string            `json:"creationTimestamp"`
	StorageLocations  []string          `json:"storageLocations,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	IsLatest          bool              `json:"isLatest"`
}

type InstanceTemplateItem struct {
	Name              string `json:"name"`
	Category          string `json:"category"`
	MachineType       string `json:"machineType"`
	DiskSizeGb        int64  `json:"diskSizeGb,omitempty"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
}

type ContainerImageSummary struct {
	PackageName string   `json:"packageName"`
	FullURI     string   `json:"fullUri"`
	Tags        []string `json:"tags"`
	Digest      string   `json:"digest"`
	SizeBytes   int64    `json:"sizeBytes"`
	UploadTime  string   `json:"uploadTime,omitempty"`
	UpdateTime  string   `json:"updateTime,omitempty"`
}

type BuildArtifactBundle struct {
	ID              string   `json:"id"`
	Bucket          string   `json:"bucket"`
	Prefix          string   `json:"prefix"`
	Category        string   `json:"category"`
	PipelineName    string   `json:"pipelineName"`
	BuildNumber     string   `json:"buildNumber"`
	TargetArch      string   `json:"targetArch,omitempty"`
	FileCount       int      `json:"fileCount"`
	TotalSizeBytes  int64    `json:"totalSizeBytes"`
	UpdatedAt       string   `json:"updatedAt"`
	KeyFiles        []string `json:"keyFiles"`
	HasGeminiAssist bool     `json:"hasGeminiAssist"`
	GCSURI          string   `json:"gcsUri"`
	ConsoleURL      string   `json:"consoleUrl"`
}

type ArchivedWorkflowItem struct {
	WorkflowName     string `json:"workflowName"`
	WorkflowTemplate string `json:"workflowTemplate"`
	Module           string `json:"module"`
	StartedAt        string `json:"startedAt"`
	FinishedAt       string `json:"finishedAt"`
	ArtifactCount    int    `json:"artifactCount"`
	TotalSizeBytes   int64  `json:"totalSizeBytes"`
	GCSURI           string `json:"gcsUri"`
	ConsoleURL       string `json:"consoleUrl"`
}

type ControlRoomABFS struct {
	Summary ABFSAccelerationSummary `json:"summary"`
	Runs    []ABFSBenchmarkRun      `json:"runs"`
}

type ABFSAccelerationSummary struct {
	CleanBaselineTotalSec       float64  `json:"cleanBaselineTotalSec"`
	CleanBaselineArm64TotalSec  float64  `json:"cleanBaselineArm64TotalSec"`
	CleanBaselineCompileSec     float64  `json:"cleanBaselineCompileSec"`
	CleanBaselineSourceDiskGb   float64  `json:"cleanBaselineSourceDiskGb"`
	CleanBaselineTotalDiskGb    float64  `json:"cleanBaselineTotalDiskGb"`
	ABFSColdSeedTotalSec        float64  `json:"abfsColdSeedTotalSec"`
	ABFSColdSeedCompileSec      float64  `json:"abfsColdSeedCompileSec"`
	ABFSColdSeedNinjaEvalSec    float64  `json:"abfsColdSeedNinjaEvalSec"`
	ABFSColdSeedSourceDiskGb    float64  `json:"abfsColdSeedSourceDiskGb"`
	ABFSColdSeedTotalDiskGb     float64  `json:"abfsColdSeedTotalDiskGb"`
	ABFSBestWarmTotalSec        float64  `json:"abfsBestWarmTotalSec"`
	ABFSAvgWarmTotalSec         float64  `json:"abfsAvgWarmTotalSec"`
	ABFSBestCompileSec          float64  `json:"abfsBestCompileSec"`
	ABFSBestNinjaEvalSec        float64  `json:"abfsBestNinjaEvalSec"`
	ABFSWarmCacheHitRatio       float64  `json:"abfsWarmCacheHitRatio"`
	ABFSWarmTotalActions        int      `json:"abfsWarmTotalActions"`
	ABFSWarmDiskFootprintGb     float64  `json:"abfsWarmDiskFootprintGb"`
	EndToEndSpeedupPeak         float64  `json:"endToEndSpeedupPeak"`
	EndToEndSpeedupAvg          float64  `json:"endToEndSpeedupAvg"`
	CompileSpeedupPeak          float64  `json:"compileSpeedupPeak"`
	NinjaEvalSpeedupPeak        float64  `json:"ninjaEvalSpeedupPeak"`
	TimeSavedPerBuildSec        float64  `json:"timeSavedPerBuildSec"`
	TimeSavedPct                float64  `json:"timeSavedPct"`
	SourceDiskReductionPct      float64  `json:"sourceDiskReductionPct"`
	TotalDiskReductionPct       float64  `json:"totalDiskReductionPct"`
	ABFSServerName              string   `json:"abfsServerName"`
	ABFSServerStatus            string   `json:"abfsServerStatus"`
	ABFSServerMachineType       string   `json:"abfsServerMachineType"`
	ABFSServerZone              string   `json:"abfsServerZone"`
	ABFSSpanNodes               []string `json:"abfsSpanNodes"`
	ABFSInfraJobsTotal          int      `json:"abfsInfraJobsTotal"`
	ABFSInfraJobsSucceeded      int      `json:"abfsInfraJobsSucceeded"`
}

type ABFSBenchmarkRun struct {
	ID                       string  `json:"id"`
	Pipeline                 string  `json:"pipeline"`
	BuildNumber              int     `json:"buildNumber"`
	Mode                     string  `json:"mode"`
	ModeLabel                string  `json:"modeLabel"`
	IsABFS                   bool    `json:"isAbfs"`
	Result                   string  `json:"result"`
	StartedAt                string  `json:"startedAt"`
	LunchTarget              string  `json:"lunchTarget"`
	Revision                 string  `json:"revision"`
	Arch                     string  `json:"arch"`
	TotalDurationSec         float64 `json:"totalDurationSec"`
	CompileDurationSec       float64 `json:"compileDurationSec,omitempty"`
	NinjaEvalSec             float64 `json:"ninjaEvalSec,omitempty"`
	InitAndSyncEstimateSec   float64 `json:"initAndSyncEstimateSec,omitempty"`
	TotalNinjaActions        int     `json:"totalNinjaActions,omitempty"`
	CacheHits                int     `json:"cacheHits,omitempty"`
	CacheMisses              int     `json:"cacheMisses,omitempty"`
	CacheHitRatio            float64 `json:"cacheHitRatio,omitempty"`
	HasRemoteCacheStats      bool    `json:"hasRemoteCacheStats"`
	BuildInfoHash            string  `json:"buildInfoHash,omitempty"`
	DiskInitialUsage         string  `json:"diskInitialUsage,omitempty"`
	DiskAfterSourceInit      string  `json:"diskAfterSourceInit,omitempty"`
	DiskAfterSourceInitGb    float64 `json:"diskAfterSourceInitGb,omitempty"`
	DiskFinalUsage           string  `json:"diskFinalUsage,omitempty"`
	DiskFinalUsageGb         float64 `json:"diskFinalUsageGb,omitempty"`
	DiskCapacity             string  `json:"diskCapacity,omitempty"`
	KernelRevision           string  `json:"kernelRevision,omitempty"`
	ABFSCachedBuild          bool    `json:"abfsCachedBuild"`
	ABFSCacheDir             string  `json:"abfsCacheDir,omitempty"`
	ManifestServer           string  `json:"manifestServer,omitempty"`
	SpeedupVsBaseline        float64 `json:"speedupVsBaseline,omitempty"`
	CompileSpeedupVsBaseline float64 `json:"compileSpeedupVsBaseline,omitempty"`
	TimeSavedSec             float64 `json:"timeSavedSec,omitempty"`
	JenkinsWebPath           string  `json:"jenkinsWebPath,omitempty"`
	GCSURI                   string  `json:"gcsUri,omitempty"`
	GCSConsoleURL            string  `json:"gcsConsoleUrl,omitempty"`
}

type parsedGCSBuildTelemetry struct {
	LunchTarget           string
	Revision              string
	Arch                  string
	KernelRevision        string
	ABFSCachedBuild       bool
	ABFSCacheDir          string
	ManifestServer        string
	DiskInitialUsage      string
	DiskAfterSourceInit   string
	DiskAfterSourceInitGb float64
	DiskFinalUsage        string
	DiskFinalUsageGb      float64
	DiskCapacity          string
	CompileDurationSec    float64
	NinjaEvalSec          float64
	TotalNinjaActions     int
	CacheHits             int
	CacheMisses           int
	CacheHitRatio         float64
	HasRemoteCacheStats   bool
	BuildInfoHash         string
}

type controlRoomService struct {
	projectID          string
	primaryRegion      string
	artifactRepo       string
	jenkinsExporterURL string
	httpClient         *http.Client

	tokenMu     sync.Mutex
	cachedToken string
	tokenExp    time.Time

	cacheMu    sync.Mutex
	cachedResp *ControlRoomResponse
	cachedAt   time.Time

	abfsLogMu    sync.Mutex
	abfsLogCache map[string]parsedGCSBuildTelemetry
}

func registerControlRoomRoutes(mux *http.ServeMux, verifier bearerVerifier) {
	svc := &controlRoomService{
		projectID:          env("GCP_PROJECT_ID", defaultGCPProjectID),
		primaryRegion:      env("GCP_REGION", defaultGCPRegion),
		artifactRepo:       env("GCP_ARTIFACT_REPO", defaultArtifactRepo),
		jenkinsExporterURL: env("JENKINS_EXPORTER_URL", defaultJenkinsExporterURL),
		httpClient: &http.Client{
			Timeout: controlRoomUpstreamTimeout,
		},
		abfsLogCache: make(map[string]parsedGCSBuildTelemetry),
	}

	mux.HandleFunc("GET /api/control-room", func(w http.ResponseWriter, r *http.Request) {
		if verifier != nil {
			raw, err := bearerToken(r)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if err := verifier.Verify(r.Context(), raw); err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		forceFresh := r.URL.Query().Get("fresh") == "1"
		payload := svc.getControlRoomSnapshot(r.Context(), forceFresh)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "private, no-store")
		_ = json.NewEncoder(w).Encode(payload)
	})
}

func (s *controlRoomService) getControlRoomSnapshot(ctx context.Context, forceFresh bool) *ControlRoomResponse {
	s.cacheMu.Lock()
	if s.cachedResp != nil {
		age := time.Since(s.cachedAt)
		ttl := controlRoomCacheTTL
		if forceFresh {
			ttl = controlRoomMinFreshInterval
		}
		if age < ttl {
			cached := s.cachedResp
			s.cacheMu.Unlock()
			return cached
		}
	}
	s.cacheMu.Unlock()

	fresh := s.collectAllTelemetry(ctx)

	s.cacheMu.Lock()
	s.cachedResp = fresh
	s.cachedAt = time.Now()
	s.cacheMu.Unlock()
	return fresh
}

func (s *controlRoomService) getGCPAccessToken(ctx context.Context) (string, error) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	if s.cachedToken != "" && time.Until(s.tokenExp) > 60*time.Second {
		return s.cachedToken, nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, gcpMetadataTokenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("metadata token request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("metadata token HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tokResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokResp); err != nil {
		return "", fmt.Errorf("decode metadata token: %w", err)
	}
	if tokResp.AccessToken == "" {
		return "", fmt.Errorf("empty access_token from metadata server")
	}
	expSec := tokResp.ExpiresIn
	if expSec <= 0 {
		expSec = 300
	}
	s.cachedToken = tokResp.AccessToken
	s.tokenExp = time.Now().Add(time.Duration(expSec) * time.Second)
	return s.cachedToken, nil
}

func (s *controlRoomService) doGCPGetJSON(ctx context.Context, gcpToken, endpoint string, target any) error {
	reqCtx, cancel := context.WithTimeout(ctx, controlRoomUpstreamTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+gcpToken)
	req.Header.Set("Accept", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, endpoint, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (s *controlRoomService) collectAllTelemetry(ctx context.Context) *ControlRoomResponse {
	out := &ControlRoomResponse{
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		ProjectID:     s.projectID,
		PrimaryRegion: s.primaryRegion,
		Workstations: ControlRoomWorkstations{
			Clusters:     []WorkstationClusterItem{},
			Configs:      []WorkstationConfigItem{},
			Workstations: []WorkstationInstanceItem{},
		},
		Compute: ControlRoomCompute{
			VirtualDevices:    []ComputeInstanceItem{},
			WorkstationVMs:    []ComputeInstanceItem{},
			BuildInfraVMs:     []ComputeInstanceItem{},
			ClusterNodes:      []ComputeInstanceItem{},
			Snapshots:         []DiskSnapshotItem{},
			InstanceTemplates: []InstanceTemplateItem{},
		},
		ArtifactImages:    []ContainerImageSummary{},
		BuildBundles:      []BuildArtifactBundle{},
		ArchivedWorkflows: []ArchivedWorkflowItem{},
		JenkinsBuilds:     []JenkinsBuildItem{},
	}

	var warnMu sync.Mutex
	addWarning := func(msg string) {
		warnMu.Lock()
		out.Warnings = append(out.Warnings, msg)
		warnMu.Unlock()
	}

	var wg sync.WaitGroup

	// 1. Fetch Jenkins builds from in-cluster exporter (independent of GCP token).
	wg.Add(1)
	go func() {
		defer wg.Done()
		builds, err := s.fetchJenkinsBuilds(ctx)
		if err != nil {
			addWarning(fmt.Sprintf("Jenkins builds exporter: %v", err))
			return
		}
		out.JenkinsBuilds = builds
	}()

	gcpToken, err := s.getGCPAccessToken(ctx)
	if err != nil {
		addWarning(fmt.Sprintf("GCP Workload Identity token: %v", err))
		wg.Wait()
		return out
	}

	// 2. Cloud Workstations (clusters, configs, instances across all regions).
	wg.Add(1)
	go func() {
		defer wg.Done()
		wsData, wsWarns := s.fetchCloudWorkstations(ctx, gcpToken)
		for _, w := range wsWarns {
			addWarning(w)
		}
		out.Workstations = wsData
	}()

	// 3. Compute Engine instances, snapshots, and instance templates.
	wg.Add(1)
	go func() {
		defer wg.Done()
		compData, compWarns := s.fetchComputeResources(ctx, gcpToken)
		for _, w := range compWarns {
			addWarning(w)
		}
		out.Compute = compData
	}()

	// 4. Artifact Registry Docker images.
	wg.Add(1)
	go func() {
		defer wg.Done()
		imgs, err := s.fetchArtifactRegistryImages(ctx, gcpToken)
		if err != nil {
			addWarning(fmt.Sprintf("Artifact Registry images: %v", err))
			return
		}
		out.ArtifactImages = imgs
	}()

	// 5. GCS Build Artifact Bundles (aaos and openbsw buckets).
	wg.Add(1)
	go func() {
		defer wg.Done()
		bundles, err := s.fetchGCSBuildBundles(ctx, gcpToken)
		if err != nil {
			addWarning(fmt.Sprintf("GCS build artifacts: %v", err))
			return
		}
		out.BuildBundles = bundles
	}()

	// 6. GCS Archived Argo Workflows (argo-workflows bucket).
	wg.Add(1)
	go func() {
		defer wg.Done()
		archived, err := s.fetchGCSArchivedWorkflows(ctx, gcpToken)
		if err != nil {
			addWarning(fmt.Sprintf("GCS archived workflows: %v", err))
			return
		}
		out.ArchivedWorkflows = archived
	}()

	wg.Wait()
	out.ABFS = s.buildABFSAccelerationTelemetry(ctx, gcpToken, out.JenkinsBuilds, out.BuildBundles, out.Compute.BuildInfraVMs)
	return out
}

func (s *controlRoomService) fetchJenkinsBuilds(ctx context.Context) ([]JenkinsBuildItem, error) {
	if strings.TrimSpace(s.jenkinsExporterURL) == "" {
		return []JenkinsBuildItem{}, nil
	}
	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, s.jenkinsExporterURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Items []JenkinsBuildItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Items == nil {
		return []JenkinsBuildItem{}, nil
	}
	return payload.Items, nil
}

func (s *controlRoomService) fetchCloudWorkstations(ctx context.Context, gcpToken string) (ControlRoomWorkstations, []string) {
	result := ControlRoomWorkstations{
		Clusters:     []WorkstationClusterItem{},
		Configs:      []WorkstationConfigItem{},
		Workstations: []WorkstationInstanceItem{},
	}
	var warnings []string
	var warnMu sync.Mutex
	addWarn := func(msg string) {
		warnMu.Lock()
		warnings = append(warnings, msg)
		warnMu.Unlock()
	}

	var wg sync.WaitGroup
	baseURL := fmt.Sprintf("https://workstations.googleapis.com/v1/projects/%s/locations/-", url.PathEscape(s.projectID))

	// Fetch clusters
	wg.Add(1)
	go func() {
		defer wg.Done()
		var raw struct {
			Clusters []struct {
				Name           string `json:"name"`
				ControlPlaneIP string `json:"controlPlaneIp"`
				Subnetwork     string `json:"subnetwork"`
				CreateTime     string `json:"createTime"`
			} `json:"workstationClusters"`
		}
		if err := s.doGCPGetJSON(ctx, gcpToken, baseURL+"/workstationClusters", &raw); err != nil {
			addWarn(fmt.Sprintf("Workstation clusters: %v", err))
			return
		}
		var items []WorkstationClusterItem
		for _, c := range raw.Clusters {
			region, clusterName, _, _ := parseWorkstationResourceName(c.Name)
			if clusterName == "" {
				clusterName = path.Base(c.Name)
			}
			items = append(items, WorkstationClusterItem{
				Name:           clusterName,
				Region:         region,
				ControlPlaneIP: c.ControlPlaneIP,
				Subnetwork:     path.Base(c.Subnetwork),
				CreateTime:     c.CreateTime,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Region != items[j].Region {
				return items[i].Region < items[j].Region
			}
			return items[i].Name < items[j].Name
		})
		result.Clusters = items
	}()

	// Fetch configs
	wg.Add(1)
	go func() {
		defer wg.Done()
		var raw struct {
			Configs []struct {
				Name           string   `json:"name"`
				IdleTimeout    string   `json:"idleTimeout"`
				RunningTimeout string   `json:"runningTimeout"`
				ReplicaZones   []string `json:"replicaZones"`
				Degraded       bool     `json:"degraded"`
				CreateTime     string   `json:"createTime"`
				Container      struct {
					Image string `json:"image"`
				} `json:"container"`
				Host struct {
					GceInstance struct {
						MachineType    string `json:"machineType"`
						BootDiskSizeGb int    `json:"bootDiskSizeGb"`
						Accelerators   []struct {
							Type  string `json:"type"`
							Count int    `json:"count"`
						} `json:"accelerators"`
					} `json:"gceInstance"`
				} `json:"host"`
				PersistentDirectories []struct {
					GcePd struct {
						SizeGb         int    `json:"sizeGb"`
						DiskType       string `json:"diskType"`
						SourceSnapshot string `json:"sourceSnapshot"`
					} `json:"gcePd"`
				} `json:"persistentDirectories"`
			} `json:"workstationConfigs"`
		}
		if err := s.doGCPGetJSON(ctx, gcpToken, baseURL+"/workstationClusters/-/workstationConfigs", &raw); err != nil {
			addWarn(fmt.Sprintf("Workstation configs: %v", err))
			return
		}
		var items []WorkstationConfigItem
		for _, cfg := range raw.Configs {
			region, clusterName, cfgName, _ := parseWorkstationResourceName(cfg.Name)
			gpuType := ""
			gpuCount := 0
			if len(cfg.Host.GceInstance.Accelerators) > 0 {
				gpuType = cfg.Host.GceInstance.Accelerators[0].Type
				gpuCount = cfg.Host.GceInstance.Accelerators[0].Count
			}
			pdSize := 0
			pdType := ""
			snap := ""
			if len(cfg.PersistentDirectories) > 0 {
				pd := cfg.PersistentDirectories[0].GcePd
				pdSize = pd.SizeGb
				pdType = pd.DiskType
				if pd.SourceSnapshot != "" {
					snap = path.Base(pd.SourceSnapshot)
					if pdSize == 0 {
						pdSize = 1000
					}
				}
			}
			items = append(items, WorkstationConfigItem{
				Name:                 cfgName,
				Region:               region,
				Cluster:              clusterName,
				MachineType:          cfg.Host.GceInstance.MachineType,
				GPUType:              gpuType,
				GPUCount:             gpuCount,
				BootDiskSizeGb:       cfg.Host.GceInstance.BootDiskSizeGb,
				PersistentDiskSizeGb: pdSize,
				PersistentDiskType:   pdType,
				SourceSnapshot:       snap,
				ContainerImage:       cfg.Container.Image,
				IdleTimeout:          cfg.IdleTimeout,
				RunningTimeout:       cfg.RunningTimeout,
				ReplicaZones:         cfg.ReplicaZones,
				Degraded:             cfg.Degraded,
				CreateTime:           cfg.CreateTime,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].Region != items[j].Region {
				return items[i].Region < items[j].Region
			}
			return items[i].Name < items[j].Name
		})
		result.Configs = items
	}()

	// Fetch workstations
	wg.Add(1)
	go func() {
		defer wg.Done()
		var raw struct {
			Workstations []struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
				State       string `json:"state"`
				Host        string `json:"host"`
				CreateTime  string `json:"createTime"`
				StartTime   string `json:"startTime"`
				UpdateTime  string `json:"updateTime"`
				RuntimeHost struct {
					GceInstanceHost struct {
						Name string `json:"name"`
						Zone string `json:"zone"`
					} `json:"gceInstanceHost"`
				} `json:"runtimeHost"`
				PersistentDirectories []struct {
					SizeGb int `json:"sizeGb"`
				} `json:"persistentDirectories"`
			} `json:"workstations"`
		}
		if err := s.doGCPGetJSON(ctx, gcpToken, baseURL+"/workstationClusters/-/workstationConfigs/-/workstations", &raw); err != nil {
			addWarn(fmt.Sprintf("Workstation instances: %v", err))
			return
		}
		var items []WorkstationInstanceItem
		for _, ws := range raw.Workstations {
			region, clusterName, cfgName, wsName := parseWorkstationResourceName(ws.Name)
			pdSize := 0
			if len(ws.PersistentDirectories) > 0 {
				pdSize = ws.PersistentDirectories[0].SizeGb
			}
			launchURL := ""
			if ws.Host != "" {
				launchURL = "https://" + ws.Host
			}
			items = append(items, WorkstationInstanceItem{
				Name:                 wsName,
				DisplayName:          ws.DisplayName,
				Region:               region,
				Cluster:              clusterName,
				Config:               cfgName,
				State:                ws.State,
				Host:                 ws.Host,
				LaunchURL:            launchURL,
				RuntimeHostVM:        ws.RuntimeHost.GceInstanceHost.Name,
				RuntimeZone:          ws.RuntimeHost.GceInstanceHost.Zone,
				PersistentDiskSizeGb: pdSize,
				CreateTime:           ws.CreateTime,
				StartTime:            ws.StartTime,
				UpdateTime:           ws.UpdateTime,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			runningI := items[i].State == "STATE_RUNNING" || items[i].State == "STATE_STARTING"
			runningJ := items[j].State == "STATE_RUNNING" || items[j].State == "STATE_STARTING"
			if runningI != runningJ {
				return runningI
			}
			return items[i].Name < items[j].Name
		})
		result.Workstations = items
	}()

	wg.Wait()

	// Enrich workstation instances with config metadata (machineType, gpuType, image, snapshot).
	cfgMap := make(map[string]WorkstationConfigItem, len(result.Configs))
	for _, cfg := range result.Configs {
		cfgMap[cfg.Region+"/"+cfg.Cluster+"/"+cfg.Name] = cfg
	}
	for i := range result.Workstations {
		ws := &result.Workstations[i]
		if cfg, ok := cfgMap[ws.Region+"/"+ws.Cluster+"/"+ws.Config]; ok {
			ws.MachineType = cfg.MachineType
			ws.GPUType = cfg.GPUType
			ws.ContainerImage = cfg.ContainerImage
			ws.SourceSnapshot = cfg.SourceSnapshot
			if ws.PersistentDiskSizeGb == 0 {
				ws.PersistentDiskSizeGb = cfg.PersistentDiskSizeGb
			}
		}
	}

	return result, warnings
}

func parseWorkstationResourceName(full string) (region, cluster, config, workstation string) {
	parts := strings.Split(full, "/")
	for i := 0; i+1 < len(parts); i++ {
		switch parts[i] {
		case "locations":
			region = parts[i+1]
		case "workstationClusters":
			cluster = parts[i+1]
		case "workstationConfigs":
			config = parts[i+1]
		case "workstations":
			workstation = parts[i+1]
		}
	}
	return region, cluster, config, workstation
}

func (s *controlRoomService) fetchComputeResources(ctx context.Context, gcpToken string) (ControlRoomCompute, []string) {
	result := ControlRoomCompute{
		VirtualDevices:    []ComputeInstanceItem{},
		WorkstationVMs:    []ComputeInstanceItem{},
		BuildInfraVMs:     []ComputeInstanceItem{},
		ClusterNodes:      []ComputeInstanceItem{},
		Snapshots:         []DiskSnapshotItem{},
		InstanceTemplates: []InstanceTemplateItem{},
	}
	var warnings []string
	var warnMu sync.Mutex
	addWarn := func(msg string) {
		warnMu.Lock()
		warnings = append(warnings, msg)
		warnMu.Unlock()
	}

	var wg sync.WaitGroup
	computeBase := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s", url.PathEscape(s.projectID))

	// 1. Aggregated instances across all zones
	wg.Add(1)
	go func() {
		defer wg.Done()
		var raw struct {
			Items map[string]struct {
				Instances []struct {
					Name              string            `json:"name"`
					Status            string            `json:"status"`
					MachineType       string            `json:"machineType"`
					Zone              string            `json:"zone"`
					CreationTimestamp string            `json:"creationTimestamp"`
					Labels            map[string]string `json:"labels"`
					NetworkInterfaces []struct {
						NetworkIP string `json:"networkIP"`
					} `json:"networkInterfaces"`
					GuestAccelerators []struct {
						AcceleratorType  string `json:"acceleratorType"`
						AcceleratorCount int    `json:"acceleratorCount"`
					} `json:"guestAccelerators"`
					Disks []struct {
						DiskSizeGb   string `json:"diskSizeGb"`
						Architecture string `json:"architecture"`
					} `json:"disks"`
				} `json:"instances"`
			} `json:"items"`
		}
		if err := s.doGCPGetJSON(ctx, gcpToken, computeBase+"/aggregated/instances", &raw); err != nil {
			addWarn(fmt.Sprintf("Compute instances: %v", err))
			return
		}
		var virtualDevs, wsVMs, buildVMs, gkeNodes []ComputeInstanceItem
		for _, scoped := range raw.Items {
			for _, inst := range scoped.Instances {
				zone := path.Base(inst.Zone)
				region := zoneToRegion(zone)
				mType := path.Base(inst.MachineType)
				ip := ""
				if len(inst.NetworkInterfaces) > 0 {
					ip = inst.NetworkInterfaces[0].NetworkIP
				}
				gpuType := ""
				gpuCount := 0
				if len(inst.GuestAccelerators) > 0 {
					gpuType = path.Base(inst.GuestAccelerators[0].AcceleratorType)
					gpuCount = inst.GuestAccelerators[0].AcceleratorCount
				}
				var diskGb int64
				arch := "X86_64"
				if len(inst.Disks) > 0 {
					diskGb, _ = strconv.ParseInt(inst.Disks[0].DiskSizeGb, 10, 64)
					if inst.Disks[0].Architecture != "" {
						arch = inst.Disks[0].Architecture
					}
				}
				if strings.Contains(strings.ToLower(inst.Name), "arm64") || strings.HasPrefix(mType, "c4a-") || strings.HasPrefix(mType, "t2a-") {
					arch = "ARM64"
				}
				role := classifyInstanceRole(inst.Name, inst.Labels)
				item := ComputeInstanceItem{
					Name:              inst.Name,
					Role:              role,
					Status:            inst.Status,
					Zone:              zone,
					Region:            region,
					MachineType:       mType,
					Architecture:      arch,
					DiskSizeGb:        diskGb,
					InternalIP:        ip,
					GPUType:           gpuType,
					GPUCount:          gpuCount,
					CreationTimestamp: inst.CreationTimestamp,
					Labels:            inst.Labels,
				}
				switch role {
				case "virtual-device":
					virtualDevs = append(virtualDevs, item)
				case "workstation-vm":
					wsVMs = append(wsVMs, item)
				case "gke-node":
					gkeNodes = append(gkeNodes, item)
				default:
					buildVMs = append(buildVMs, item)
				}
			}
		}
		sortComputeInstances(virtualDevs)
		sortComputeInstances(wsVMs)
		sortComputeInstances(buildVMs)
		sortComputeInstances(gkeNodes)
		result.VirtualDevices = virtualDevs
		result.WorkstationVMs = wsVMs
		result.BuildInfraVMs = buildVMs
		result.ClusterNodes = gkeNodes
	}()

	// 2. Snapshots
	wg.Add(1)
	go func() {
		defer wg.Done()
		var raw struct {
			Items []struct {
				Name              string            `json:"name"`
				Status            string            `json:"status"`
				DiskSizeGb        string            `json:"diskSizeGb"`
				StorageBytes      string            `json:"storageBytes"`
				CreationTimestamp string            `json:"creationTimestamp"`
				StorageLocations  []string          `json:"storageLocations"`
				Labels            map[string]string `json:"labels"`
			} `json:"items"`
		}
		if err := s.doGCPGetJSON(ctx, gcpToken, computeBase+"/global/snapshots", &raw); err != nil {
			addWarn(fmt.Sprintf("Compute snapshots: %v", err))
			return
		}
		var snaps []DiskSnapshotItem
		for _, sn := range raw.Items {
			diskGb, _ := strconv.ParseInt(sn.DiskSizeGb, 10, 64)
			storageBytes, _ := strconv.ParseInt(sn.StorageBytes, 10, 64)
			snaps = append(snaps, DiskSnapshotItem{
				Name:              sn.Name,
				Status:            sn.Status,
				DiskSizeGb:        diskGb,
				StorageBytes:      storageBytes,
				CreationTimestamp: sn.CreationTimestamp,
				StorageLocations:  sn.StorageLocations,
				Labels:            sn.Labels,
				IsLatest:          sn.Labels["is-latest"] == "true",
			})
		}
		sort.Slice(snaps, func(i, j int) bool {
			if snaps[i].IsLatest != snaps[j].IsLatest {
				return snaps[i].IsLatest
			}
			return snaps[i].CreationTimestamp > snaps[j].CreationTimestamp
		})
		result.Snapshots = snaps
	}()

	// 3. Instance Templates
	wg.Add(1)
	go func() {
		defer wg.Done()
		var raw struct {
			Items []struct {
				Name              string `json:"name"`
				CreationTimestamp string `json:"creationTimestamp"`
				Properties        struct {
					MachineType string `json:"machineType"`
					Disks       []struct {
						InitializeParams struct {
							DiskSizeGb string `json:"diskSizeGb"`
						} `json:"initializeParams"`
					} `json:"disks"`
				} `json:"properties"`
			} `json:"items"`
		}
		if err := s.doGCPGetJSON(ctx, gcpToken, computeBase+"/global/instanceTemplates", &raw); err != nil {
			addWarn(fmt.Sprintf("Instance templates: %v", err))
			return
		}
		var tpls []InstanceTemplateItem
		for _, t := range raw.Items {
			var diskGb int64
			if len(t.Properties.Disks) > 0 {
				diskGb, _ = strconv.ParseInt(t.Properties.Disks[0].InitializeParams.DiskSizeGb, 10, 64)
			}
			cat := "compute"
			lower := strings.ToLower(t.Name)
			if strings.Contains(lower, "cuttlefish") || strings.Contains(lower, "remotive") {
				cat = "virtual-device"
			} else if strings.HasPrefix(lower, "gke-") {
				cat = "gke-node-pool"
			}
			tpls = append(tpls, InstanceTemplateItem{
				Name:              t.Name,
				Category:          cat,
				MachineType:       t.Properties.MachineType,
				DiskSizeGb:        diskGb,
				CreationTimestamp: t.CreationTimestamp,
			})
		}
		sort.Slice(tpls, func(i, j int) bool {
			if (tpls[i].Category == "virtual-device") != (tpls[j].Category == "virtual-device") {
				return tpls[i].Category == "virtual-device"
			}
			return tpls[i].Name < tpls[j].Name
		})
		result.InstanceTemplates = tpls
	}()

	wg.Wait()
	return result, warnings
}

func classifyInstanceRole(name string, labels map[string]string) string {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "cuttlefish-") || strings.HasPrefix(lower, "remotive-") {
		return "virtual-device"
	}
	if cfg, ok := labels["jenkins_config_name"]; ok {
		lcfg := strings.ToLower(cfg)
		if strings.Contains(lcfg, "cuttlefish") || strings.Contains(lcfg, "remotive") {
			return "virtual-device"
		}
	}
	if strings.HasPrefix(lower, "workstations-") {
		return "workstation-vm"
	}
	if strings.HasPrefix(lower, "gke-") {
		return "gke-node"
	}
	return "build-infra"
}

func zoneToRegion(zone string) string {
	parts := strings.Split(zone, "-")
	if len(parts) >= 3 {
		return strings.Join(parts[:len(parts)-1], "-")
	}
	return zone
}

func sortComputeInstances(items []ComputeInstanceItem) {
	sort.Slice(items, func(i, j int) bool {
		runI := items[i].Status == "RUNNING"
		runJ := items[j].Status == "RUNNING"
		if runI != runJ {
			return runI
		}
		return items[i].Name < items[j].Name
	})
}

func (s *controlRoomService) fetchArtifactRegistryImages(ctx context.Context, gcpToken string) ([]ContainerImageSummary, error) {
	endpoint := fmt.Sprintf(
		"https://artifactregistry.googleapis.com/v1/projects/%s/locations/%s/repositories/%s/dockerImages?pageSize=200",
		url.PathEscape(s.projectID),
		url.PathEscape(s.primaryRegion),
		url.PathEscape(s.artifactRepo),
	)
	var raw struct {
		DockerImages []struct {
			Name           string   `json:"name"`
			URI            string   `json:"uri"`
			Tags           []string `json:"tags"`
			ImageSizeBytes string   `json:"imageSizeBytes"`
			UploadTime     string   `json:"uploadTime"`
			UpdateTime     string   `json:"updateTime"`
		} `json:"dockerImages"`
	}
	if err := s.doGCPGetJSON(ctx, gcpToken, endpoint, &raw); err != nil {
		return nil, err
	}

	// Keep all tagged images, and for packages that only have untagged images keep the most recent one.
	byPkgLatest := make(map[string]ContainerImageSummary)
	var taggedList []ContainerImageSummary

	repoPrefix := fmt.Sprintf("%s-docker.pkg.dev/%s/%s/", s.primaryRegion, s.projectID, s.artifactRepo)
	for _, img := range raw.DockerImages {
		uriWithoutDigest := img.URI
		digest := ""
		if atIdx := strings.LastIndex(img.URI, "@"); atIdx > 0 {
			uriWithoutDigest = img.URI[:atIdx]
			digest = img.URI[atIdx+1:]
		}
		pkgName := strings.TrimPrefix(uriWithoutDigest, repoPrefix)
		if decoded, err := url.PathUnescape(pkgName); err == nil {
			pkgName = decoded
		}
		sizeBytes, _ := strconv.ParseInt(img.ImageSizeBytes, 10, 64)
		summary := ContainerImageSummary{
			PackageName: pkgName,
			FullURI:     uriWithoutDigest,
			Tags:        img.Tags,
			Digest:      digest,
			SizeBytes:   sizeBytes,
			UploadTime:  img.UploadTime,
			UpdateTime:  img.UpdateTime,
		}
		if len(img.Tags) > 0 {
			taggedList = append(taggedList, summary)
			continue
		}
		prev, exists := byPkgLatest[pkgName]
		if !exists || summary.UpdateTime > prev.UpdateTime {
			byPkgLatest[pkgName] = summary
		}
	}

	seenTaggedPkg := make(map[string]bool)
	for _, t := range taggedList {
		seenTaggedPkg[t.PackageName] = true
	}
	for pkg, untagged := range byPkgLatest {
		if !seenTaggedPkg[pkg] {
			taggedList = append(taggedList, untagged)
		}
	}

	sort.Slice(taggedList, func(i, j int) bool {
		if taggedList[i].UpdateTime != taggedList[j].UpdateTime {
			return taggedList[i].UpdateTime > taggedList[j].UpdateTime
		}
		return taggedList[i].PackageName < taggedList[j].PackageName
	})
	return taggedList, nil
}

type gcsObjectItem struct {
	Name        string `json:"name"`
	Size        string `json:"size"`
	TimeCreated string `json:"timeCreated"`
	Updated     string `json:"updated"`
}

func (s *controlRoomService) listGCSBucketObjects(ctx context.Context, gcpToken, bucket string) ([]gcsObjectItem, error) {
	endpoint := fmt.Sprintf("https://storage.googleapis.com/storage/v1/b/%s/o?maxResults=600&fields=items(name,size,timeCreated,updated)", url.PathEscape(bucket))
	var raw struct {
		Items []gcsObjectItem `json:"items"`
	}
	if err := s.doGCPGetJSON(ctx, gcpToken, endpoint, &raw); err != nil {
		return nil, err
	}
	return raw.Items, nil
}

func (s *controlRoomService) fetchGCSBuildBundles(ctx context.Context, gcpToken string) ([]BuildArtifactBundle, error) {
	buckets := []string{
		s.projectID + "-aaos",
		s.projectID + "-openbsw",
	}
	bundleMap := make(map[string]*BuildArtifactBundle)

	for _, bucket := range buckets {
		objs, err := s.listGCSBucketObjects(ctx, gcpToken, bucket)
		if err != nil {
			continue
		}
		for _, obj := range objs {
			if strings.HasSuffix(obj.Name, "/") {
				continue
			}
			parts := strings.Split(obj.Name, "/")
			if len(parts) < 3 {
				continue
			}
			var prefix, category, pipelineName, buildNum string
			if parts[0] == "RemotiveTopology" && len(parts) >= 3 && parts[1] == "Remotive_Launcher_Workflows" {
				prefix = strings.Join(parts[:3], "/")
				category = "RemotiveTopology"
				pipelineName = "Remotive Launcher"
				buildNum = parts[2]
			} else if len(parts) >= 4 {
				prefix = strings.Join(parts[:4], "/")
				category = parts[0] + " / " + parts[1]
				pipelineName = strings.ReplaceAll(parts[2], "_", " ")
				buildNum = parts[3]
			} else {
				continue
			}

			key := bucket + "/" + prefix
			b, exists := bundleMap[key]
			if !exists {
				b = &BuildArtifactBundle{
					ID:           key,
					Bucket:       bucket,
					Prefix:       prefix,
					Category:     category,
					PipelineName: pipelineName,
					BuildNumber:  buildNum,
					KeyFiles:     []string{},
					GCSURI:       fmt.Sprintf("gs://%s/%s/", bucket, prefix),
					ConsoleURL:   fmt.Sprintf("https://console.cloud.google.com/storage/browser/%s/%s?project=%s", url.PathEscape(bucket), prefix, url.QueryEscape(s.projectID)),
				}
				bundleMap[key] = b
			}
			sz, _ := strconv.ParseInt(obj.Size, 10, 64)
			b.FileCount++
			b.TotalSizeBytes += sz
			ts := obj.Updated
			if ts == "" {
				ts = obj.TimeCreated
			}
			if ts > b.UpdatedAt {
				b.UpdatedAt = ts
			}
			relFile := strings.TrimPrefix(obj.Name, prefix+"/")
			if strings.HasPrefix(relFile, "gemini-assist/") {
				b.HasGeminiAssist = true
			}
			inferBundleArch(b, relFile)
			if isNotableBuildFile(relFile) && len(b.KeyFiles) < 6 {
				b.KeyFiles = append(b.KeyFiles, path.Base(relFile))
			}
		}
	}

	var list []BuildArtifactBundle
	for _, b := range bundleMap {
		list = append(list, *b)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].UpdatedAt != list[j].UpdatedAt {
			return list[i].UpdatedAt > list[j].UpdatedAt
		}
		return list[i].ID > list[j].ID
	})
	return list, nil
}

func inferBundleArch(b *BuildArtifactBundle, relFile string) {
	lower := strings.ToLower(relFile)
	switch {
	case strings.Contains(lower, "arm64"):
		b.TargetArch = "ARM64"
	case strings.Contains(lower, "x86_64"):
		b.TargetArch = "x86_64"
	case strings.Contains(lower, "s32k148"):
		b.TargetArch = "NXP S32K148 + POSIX"
	case strings.Contains(lower, "sdv_core_cf"):
		if b.TargetArch == "" {
			b.TargetArch = "SDV Core Cuttlefish"
		}
	}
}

func isNotableBuildFile(relFile string) bool {
	if strings.HasPrefix(relFile, "headless_output_") || strings.HasPrefix(relFile, "ephemeral-input/") {
		return false
	}
	base := strings.ToLower(path.Base(relFile))
	for _, suffix := range []string{".zip", ".tar.gz", ".tgz", ".apk", ".elf", "sbom.spdx.json", "build-info.txt", ".log"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func (s *controlRoomService) fetchGCSArchivedWorkflows(ctx context.Context, gcpToken string) ([]ArchivedWorkflowItem, error) {
	bucket := s.projectID + "-argo-workflows"
	objs, err := s.listGCSBucketObjects(ctx, gcpToken, bucket)
	if err != nil {
		return nil, err
	}
	byWf := make(map[string]*ArchivedWorkflowItem)
	for _, obj := range objs {
		if strings.HasSuffix(obj.Name, "/") {
			continue
		}
		parts := strings.Split(obj.Name, "/")
		if len(parts) < 2 {
			continue
		}
		wfName := parts[0]
		if wfName == "control-room" {
			continue
		}
		item, exists := byWf[wfName]
		if !exists {
			tpl, mod := inferWorkflowTemplateAndModule(wfName)
			item = &ArchivedWorkflowItem{
				WorkflowName:     wfName,
				WorkflowTemplate: tpl,
				Module:           mod,
				GCSURI:           fmt.Sprintf("gs://%s/%s/", bucket, wfName),
				ConsoleURL:       fmt.Sprintf("https://console.cloud.google.com/storage/browser/%s/%s?project=%s", url.PathEscape(bucket), url.PathEscape(wfName), url.QueryEscape(s.projectID)),
			}
			byWf[wfName] = item
		}
		sz, _ := strconv.ParseInt(obj.Size, 10, 64)
		item.ArtifactCount++
		item.TotalSizeBytes += sz
		ts := obj.TimeCreated
		if ts == "" {
			ts = obj.Updated
		}
		if ts != "" {
			if item.StartedAt == "" || ts < item.StartedAt {
				item.StartedAt = ts
			}
			if item.FinishedAt == "" || ts > item.FinishedAt {
				item.FinishedAt = ts
			}
		}
	}
	var list []ArchivedWorkflowItem
	for _, v := range byWf {
		list = append(list, *v)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].FinishedAt != list[j].FinishedAt {
			return list[i].FinishedAt > list[j].FinishedAt
		}
		return list[i].WorkflowName > list[j].WorkflowName
	})
	return list, nil
}

func inferWorkflowTemplateAndModule(wfName string) (string, string) {
	trimmed := strings.TrimPrefix(wfName, "webhook-")
	lastDash := strings.LastIndex(trimmed, "-")
	tpl := trimmed
	if lastDash > 0 {
		tpl = trimmed[:lastDash]
	}
	switch {
	case strings.HasPrefix(tpl, "aaos-sdv-dev-env"):
		return tpl, "aaos-sdv-dev-env"
	case strings.HasPrefix(tpl, "remotive-"):
		return tpl, "remotive-topology"
	case strings.HasPrefix(tpl, "aaos-") || strings.HasPrefix(tpl, "cf-") || strings.HasPrefix(tpl, "cvd-") || strings.HasPrefix(tpl, "cts-"):
		return tpl, "workloads-android"
	case strings.HasPrefix(tpl, "storage-gcs"):
		return tpl, "storage-gcs"
	default:
		return tpl, "workloads-common"
	}
}

var (
	reStorageUsage    = regexp.MustCompile(`Storage Usage \(/aaos-cache\):\s*Used\s+([0-9.]+[KMGT]?)\s+of\s+([0-9.]+[KMGT]?)`)
	reKernelRevision  = regexp.MustCompile(`Kernel Revision:\s*([^\s\r\n]+)`)
	reBuildCompleted  = regexp.MustCompile(`#### build completed successfully \(([^)]+)\) ####`)
	reABFSNinjaEval   = regexp.MustCompile(`Ninja evaluation took ([0-9hms.]+), fail:\s*\d+,\s*skip:\s*\d+,\s*cache:\s*hits:\s*(\d+),\s*misses:\s*(\d+),\s*forces:\s*\d+,\s*ratio:\s*([0-9.]+)`)
	reABFSBuildHash   = regexp.MustCompile(`Saved BuildInfo for \d+ with hash ([0-9a-f]+)`)
	reNinjaFullSteps  = regexp.MustCompile(`\[100%\s+(\d+)/(\d+)[^\]]*\]`)
	reSisoNinjaSteps  = regexp.MustCompile(`ninja:\s+([0-9hms.]+)\s+Build Succeeded:\s+(\d+)\s+steps`)
)

func (s *controlRoomService) buildABFSAccelerationTelemetry(
	ctx context.Context,
	gcpToken string,
	jenkinsBuilds []JenkinsBuildItem,
	bundles []BuildArtifactBundle,
	infraVMs []ComputeInstanceItem,
) ControlRoomABFS {
	bundleByKey := make(map[string]BuildArtifactBundle)
	var aaosBundles []BuildArtifactBundle
	for _, b := range bundles {
		if strings.HasPrefix(b.Prefix, "Android/Builds/") {
			aaosBundles = append(aaosBundles, b)
			num, err := strconv.Atoi(strings.TrimLeft(b.BuildNumber, "0"))
			if err != nil && b.BuildNumber == "0" {
				num = 0
			}
			key := fmt.Sprintf("%s#%d", b.PipelineName, num)
			bundleByKey[key] = b
		}
	}

	// Ensure GCS build-info and log tails are parsed and cached in parallel.
	var logWg sync.WaitGroup
	for _, b := range aaosBundles {
		cacheKey := fmt.Sprintf("%s/%s@%s", b.Bucket, b.Prefix, b.UpdatedAt)
		s.abfsLogMu.Lock()
		_, ok := s.abfsLogCache[cacheKey]
		s.abfsLogMu.Unlock()
		if ok {
			continue
		}
		logWg.Add(1)
		go func(bundle BuildArtifactBundle, key string) {
			defer logWg.Done()
			parsed := s.fetchAndParseGCSBuildTelemetry(ctx, gcpToken, bundle)
			s.abfsLogMu.Lock()
			s.abfsLogCache[key] = parsed
			s.abfsLogMu.Unlock()
		}(b, cacheKey)
	}
	logWg.Wait()

	var runs []ABFSBenchmarkRun
	abfsInfraTotal := 0
	abfsInfraSuccess := 0

	for _, jb := range jenkinsBuilds {
		if strings.HasPrefix(jb.JobPath, "Android / Environment / ABFS") {
			abfsInfraTotal++
			if jb.Result == "SUCCESS" {
				abfsInfraSuccess++
			}
		}

		var pipelineKey, pipelineLabel string
		isABFS := false
		switch jb.JobPath {
		case "Android / Builds / AAOS Builder ABFS":
			pipelineKey = "AAOS_Builder_ABFS"
			pipelineLabel = "AAOS Builder ABFS"
			isABFS = true
		case "Android / Builds / AAOS Builder":
			pipelineKey = "AAOS_Builder"
			pipelineLabel = "AAOS Builder (Standard)"
		case "Android / Builds / AAOS SDV Builder":
			pipelineKey = "AAOS_SDV_Builder"
			pipelineLabel = "AAOS SDV Builder"
		default:
			continue
		}

		bKey := fmt.Sprintf("%s#%d", pipelineKey, jb.BuildNumber)
		bundle, hasBundle := bundleByKey[bKey]
		var gcsTelem parsedGCSBuildTelemetry
		if hasBundle {
			cKey := fmt.Sprintf("%s/%s@%s", bundle.Bucket, bundle.Prefix, bundle.UpdatedAt)
			s.abfsLogMu.Lock()
			gcsTelem = s.abfsLogCache[cKey]
			s.abfsLogMu.Unlock()
		}

		target := jb.Parameters["AAOS_LUNCH_TARGET"]
		if target == "" {
			target = gcsTelem.LunchTarget
		}
		rev := jb.Parameters["AAOS_REVISION"]
		if rev == "" {
			rev = gcsTelem.Revision
		}
		arch := gcsTelem.Arch
		if arch == "" {
			if strings.Contains(target, "arm64") {
				arch = "arm64"
			} else if target != "" {
				arch = "x86_64"
			}
		}

		totalSec := float64(jb.DurationMs) / 1000.0
		compileSec := gcsTelem.CompileDurationSec
		initAndSyncSec := 0.0
		if totalSec > compileSec && compileSec > 0 {
			initAndSyncSec = totalSec - compileSec
		}

		mode := "standard-clean"
		modeLabel := "Standard Clean (Repo Sync)"
		switch pipelineKey {
		case "AAOS_Builder_ABFS":
			if (gcsTelem.HasRemoteCacheStats && gcsTelem.CacheHits == 0 && gcsTelem.CacheMisses > 0) || totalSec > 1800 {
				mode = "abfs-seed"
				modeLabel = "ABFS Cache Seed (Cold CAS)"
			} else {
				mode = "abfs-warm"
				modeLabel = "ABFS Accelerated (Warm CAS)"
			}
		case "AAOS_Builder":
			if totalSec >= 2400 {
				mode = "standard-clean"
				modeLabel = "Standard Clean (Repo Sync)"
			} else {
				mode = "standard-warm"
				modeLabel = "Standard Incremental (Warm PVC)"
			}
		case "AAOS_SDV_Builder":
			if totalSec >= 1800 {
				mode = "sdv-clean"
				modeLabel = "SDV Clean (Repo Sync + Cold Siso)"
			} else {
				mode = "sdv-warm"
				modeLabel = "SDV Incremental (Warm PVC Siso)"
			}
		}

		run := ABFSBenchmarkRun{
			ID:                     jb.ID,
			Pipeline:               pipelineLabel,
			BuildNumber:            jb.BuildNumber,
			Mode:                   mode,
			ModeLabel:              modeLabel,
			IsABFS:                 isABFS,
			Result:                 jb.Result,
			StartedAt:              jb.StartedAt,
			LunchTarget:            target,
			Revision:               rev,
			Arch:                   arch,
			TotalDurationSec:       round1(totalSec),
			CompileDurationSec:     round1(compileSec),
			NinjaEvalSec:           round1(gcsTelem.NinjaEvalSec),
			InitAndSyncEstimateSec: round1(initAndSyncSec),
			TotalNinjaActions:      gcsTelem.TotalNinjaActions,
			CacheHits:              gcsTelem.CacheHits,
			CacheMisses:            gcsTelem.CacheMisses,
			CacheHitRatio:          round1(gcsTelem.CacheHitRatio),
			HasRemoteCacheStats:    gcsTelem.HasRemoteCacheStats,
			BuildInfoHash:          gcsTelem.BuildInfoHash,
			DiskInitialUsage:       gcsTelem.DiskInitialUsage,
			DiskAfterSourceInit:    gcsTelem.DiskAfterSourceInit,
			DiskAfterSourceInitGb:  round2(gcsTelem.DiskAfterSourceInitGb),
			DiskFinalUsage:         gcsTelem.DiskFinalUsage,
			DiskFinalUsageGb:       round2(gcsTelem.DiskFinalUsageGb),
			DiskCapacity:           gcsTelem.DiskCapacity,
			KernelRevision:         gcsTelem.KernelRevision,
			ABFSCachedBuild:        gcsTelem.ABFSCachedBuild || jb.Parameters["ABFS_CACHED_BUILD"] == "true",
			ABFSCacheDir:           gcsTelem.ABFSCacheDir,
			ManifestServer:         gcsTelem.ManifestServer,
			JenkinsWebPath:         jb.WebPath,
		}
		if hasBundle {
			run.GCSURI = bundle.GCSURI
			run.GCSConsoleURL = bundle.ConsoleURL
		}
		runs = append(runs, run)
	}

	// Determine clean standard baselines for speedup calculation.
	var (
		cleanX86TotalSec    = 4874.3
		cleanArm64TotalSec  = 5243.1
		cleanX86CompileSec  = 2854.0
		cleanSourceDiskGb   = 131.0
		cleanTotalDiskGb    = 274.0
		sdvCleanTotalSec    = 3155.3
		sdvCleanCompileSec  = 1751.0
		abfsSeedTotalSec    = 5148.5
		abfsSeedCompileSec  = 3781.0
		abfsSeedNinjaSec    = 3551.0
		abfsSeedSourceGb    = 2.2
		abfsSeedTotalDiskGb = 162.0
		abfsBestWarmSec     = 0.0
		abfsWarmSumSec      = 0.0
		abfsWarmCount       = 0
		abfsBestCompileSec  = 0.0
		abfsBestNinjaSec    = 0.0
		abfsWarmHitRatio    = 100.0
		abfsWarmActions     = 181251
		abfsWarmMinDiskGb   = 0.0
	)

	for _, r := range runs {
		if r.Result != "SUCCESS" {
			continue
		}
		switch r.Mode {
		case "standard-clean":
			if r.Arch == "arm64" {
				cleanArm64TotalSec = r.TotalDurationSec
			} else {
				cleanX86TotalSec = r.TotalDurationSec
				if r.CompileDurationSec > 0 {
					cleanX86CompileSec = r.CompileDurationSec
				}
				if r.DiskAfterSourceInitGb > 0 {
					cleanSourceDiskGb = r.DiskAfterSourceInitGb
				}
				if r.DiskFinalUsageGb > 0 {
					cleanTotalDiskGb = r.DiskFinalUsageGb
				}
			}
		case "sdv-clean":
			sdvCleanTotalSec = r.TotalDurationSec
			if r.CompileDurationSec > 0 {
				sdvCleanCompileSec = r.CompileDurationSec
			}
		case "abfs-seed":
			abfsSeedTotalSec = r.TotalDurationSec
			if r.CompileDurationSec > 0 {
				abfsSeedCompileSec = r.CompileDurationSec
			}
			if r.NinjaEvalSec > 0 {
				abfsSeedNinjaSec = r.NinjaEvalSec
			}
			if r.DiskAfterSourceInitGb > 0 {
				abfsSeedSourceGb = r.DiskAfterSourceInitGb
			}
			if r.DiskFinalUsageGb > 0 {
				abfsSeedTotalDiskGb = r.DiskFinalUsageGb
			}
		case "abfs-warm":
			abfsWarmCount++
			abfsWarmSumSec += r.TotalDurationSec
			if abfsBestWarmSec == 0 || r.TotalDurationSec < abfsBestWarmSec {
				abfsBestWarmSec = r.TotalDurationSec
			}
			if r.CompileDurationSec > 0 && (abfsBestCompileSec == 0 || r.CompileDurationSec < abfsBestCompileSec) {
				abfsBestCompileSec = r.CompileDurationSec
			}
			if r.NinjaEvalSec > 0 && (abfsBestNinjaSec == 0 || r.NinjaEvalSec < abfsBestNinjaSec) {
				abfsBestNinjaSec = r.NinjaEvalSec
			}
			if r.HasRemoteCacheStats {
				abfsWarmHitRatio = r.CacheHitRatio
				if r.TotalNinjaActions > 0 {
					abfsWarmActions = r.TotalNinjaActions
				}
			}
			if r.DiskFinalUsageGb > 0 && (abfsWarmMinDiskGb == 0 || r.DiskFinalUsageGb < abfsWarmMinDiskGb) {
				abfsWarmMinDiskGb = r.DiskFinalUsageGb
			}
		}
	}

	if abfsBestWarmSec == 0 {
		abfsBestWarmSec = 370.0
	}
	if abfsBestCompileSec == 0 {
		abfsBestCompileSec = 99.0
	}
	if abfsBestNinjaSec == 0 {
		abfsBestNinjaSec = 46.2
	}
	if abfsWarmMinDiskGb == 0 {
		abfsWarmMinDiskGb = 19.0
	}
	abfsAvgWarmSec := abfsBestWarmSec
	if abfsWarmCount > 0 {
		abfsAvgWarmSec = abfsWarmSumSec / float64(abfsWarmCount)
	}

	// Enrich each successful run with speedup vs its corresponding clean baseline.
	for i := range runs {
		r := &runs[i]
		if r.Result != "SUCCESS" || r.TotalDurationSec <= 0 {
			continue
		}
		baseTotal := cleanX86TotalSec
		baseCompile := cleanX86CompileSec
		if r.Arch == "arm64" {
			baseTotal = cleanArm64TotalSec
		} else if strings.HasPrefix(r.Mode, "sdv-") {
			baseTotal = sdvCleanTotalSec
			baseCompile = sdvCleanCompileSec
		}
		if baseTotal > 0 {
			r.SpeedupVsBaseline = round1(baseTotal / r.TotalDurationSec)
			if baseTotal > r.TotalDurationSec {
				r.TimeSavedSec = round1(baseTotal - r.TotalDurationSec)
			}
		}
		if baseCompile > 0 && r.CompileDurationSec > 0 {
			r.CompileSpeedupVsBaseline = round1(baseCompile / r.CompileDurationSec)
		}
	}

	abfsServerName := "abfs-server-main-0"
	abfsServerStatus := "UNKNOWN"
	abfsServerMT := ""
	abfsServerZone := ""
	var spanNodes []string
	for _, vm := range infraVMs {
		lower := strings.ToLower(vm.Name)
		if strings.HasPrefix(lower, "abfs-server") {
			abfsServerName = vm.Name
			abfsServerStatus = vm.Status
			abfsServerMT = vm.MachineType
			abfsServerZone = vm.Zone
		} else if strings.HasPrefix(lower, "abfs-span") {
			spanNodes = append(spanNodes, vm.Name)
		}
	}
	sort.Strings(spanNodes)

	e2ePeak := round1(cleanX86TotalSec / abfsBestWarmSec)
	e2eAvg := round1(cleanX86TotalSec / abfsAvgWarmSec)
	compilePeak := round1(cleanX86CompileSec / abfsBestCompileSec)
	ninjaPeak := round1(cleanX86CompileSec / abfsBestNinjaSec)
	savedSec := round1(cleanX86TotalSec - abfsBestWarmSec)
	savedPct := round1(((cleanX86TotalSec - abfsBestWarmSec) / cleanX86TotalSec) * 100.0)
	srcDiskRedPct := round1(((cleanSourceDiskGb - abfsSeedSourceGb) / cleanSourceDiskGb) * 100.0)
	totDiskRedPct := round1(((cleanTotalDiskGb - abfsWarmMinDiskGb) / cleanTotalDiskGb) * 100.0)

	return ControlRoomABFS{
		Summary: ABFSAccelerationSummary{
			CleanBaselineTotalSec:      round1(cleanX86TotalSec),
			CleanBaselineArm64TotalSec: round1(cleanArm64TotalSec),
			CleanBaselineCompileSec:    round1(cleanX86CompileSec),
			CleanBaselineSourceDiskGb:  round2(cleanSourceDiskGb),
			CleanBaselineTotalDiskGb:   round2(cleanTotalDiskGb),
			ABFSColdSeedTotalSec:       round1(abfsSeedTotalSec),
			ABFSColdSeedCompileSec:     round1(abfsSeedCompileSec),
			ABFSColdSeedNinjaEvalSec:   round1(abfsSeedNinjaSec),
			ABFSColdSeedSourceDiskGb:   round2(abfsSeedSourceGb),
			ABFSColdSeedTotalDiskGb:    round2(abfsSeedTotalDiskGb),
			ABFSBestWarmTotalSec:       round1(abfsBestWarmSec),
			ABFSAvgWarmTotalSec:        round1(abfsAvgWarmSec),
			ABFSBestCompileSec:         round1(abfsBestCompileSec),
			ABFSBestNinjaEvalSec:       round1(abfsBestNinjaSec),
			ABFSWarmCacheHitRatio:      round1(abfsWarmHitRatio),
			ABFSWarmTotalActions:       abfsWarmActions,
			ABFSWarmDiskFootprintGb:    round2(abfsWarmMinDiskGb),
			EndToEndSpeedupPeak:        e2ePeak,
			EndToEndSpeedupAvg:         e2eAvg,
			CompileSpeedupPeak:         compilePeak,
			NinjaEvalSpeedupPeak:       ninjaPeak,
			TimeSavedPerBuildSec:       savedSec,
			TimeSavedPct:               savedPct,
			SourceDiskReductionPct:     srcDiskRedPct,
			TotalDiskReductionPct:      totDiskRedPct,
			ABFSServerName:             abfsServerName,
			ABFSServerStatus:           abfsServerStatus,
			ABFSServerMachineType:      abfsServerMT,
			ABFSServerZone:             abfsServerZone,
			ABFSSpanNodes:              spanNodes,
			ABFSInfraJobsTotal:         abfsInfraTotal,
			ABFSInfraJobsSucceeded:     abfsInfraSuccess,
		},
		Runs: runs,
	}
}

func (s *controlRoomService) fetchAndParseGCSBuildTelemetry(ctx context.Context, gcpToken string, bundle BuildArtifactBundle) parsedGCSBuildTelemetry {
	var out parsedGCSBuildTelemetry
	infoPath := strings.TrimSuffix(bundle.Prefix, "/") + "/aaos-build-info.txt"
	if infoText, err := s.fetchGCSObjectMedia(ctx, gcpToken, bundle.Bucket, infoPath, ""); err == nil && infoText != "" {
		parseBuildInfoText(infoText, &out)
	}
	logPath := strings.TrimSuffix(bundle.Prefix, "/") + "/aaos-build.log"
	if logTail, err := s.fetchGCSObjectMedia(ctx, gcpToken, bundle.Bucket, logPath, "bytes=-6144"); err == nil && logTail != "" {
		parseBuildLogTail(logTail, &out)
	}
	return out
}

func (s *controlRoomService) fetchGCSObjectMedia(ctx context.Context, gcpToken, bucket, objectName, byteRange string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	endpoint := fmt.Sprintf(
		"https://storage.googleapis.com/storage/v1/b/%s/o/%s?alt=media",
		url.PathEscape(bucket),
		url.PathEscape(objectName),
	)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+gcpToken)
	if byteRange != "" {
		req.Header.Set("Range", byteRange)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32768))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func parseBuildInfoText(text string, out *parsedGCSBuildTelemetry) {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "AAOS_LUNCH_TARGET=") && out.LunchTarget == "":
			out.LunchTarget = strings.TrimSpace(strings.TrimPrefix(trimmed, "AAOS_LUNCH_TARGET="))
		case strings.HasPrefix(trimmed, "AAOS_REVISION=") && out.Revision == "":
			out.Revision = strings.TrimSpace(strings.TrimPrefix(trimmed, "AAOS_REVISION="))
		case strings.HasPrefix(trimmed, "AAOS_ARCH=") && out.Arch == "":
			out.Arch = strings.TrimSpace(strings.TrimPrefix(trimmed, "AAOS_ARCH="))
		case strings.HasPrefix(trimmed, "ABFS_CACHED_BUILD=true"):
			out.ABFSCachedBuild = true
		case strings.HasPrefix(trimmed, "ABFS_CMD_FLAGS="):
			flags := strings.TrimSpace(strings.TrimPrefix(trimmed, "ABFS_CMD_FLAGS="))
			if strings.HasPrefix(flags, "--cache-dir ") {
				out.ABFSCacheDir = strings.TrimSpace(strings.TrimPrefix(flags, "--cache-dir "))
			} else {
				out.ABFSCacheDir = flags
			}
		case strings.HasPrefix(trimmed, "UPLOADER_MANIFEST_SERVER=") && out.ManifestServer == "":
			out.ManifestServer = strings.TrimSpace(strings.TrimPrefix(trimmed, "UPLOADER_MANIFEST_SERVER="))
		case strings.HasPrefix(trimmed, "GERRIT_SERVER_URL=") && out.ManifestServer == "":
			out.ManifestServer = strings.TrimSpace(strings.TrimPrefix(trimmed, "GERRIT_SERVER_URL="))
		}
	}

	if km := reKernelRevision.FindStringSubmatch(text); len(km) >= 2 {
		out.KernelRevision = strings.TrimSpace(km[1])
	}

	usages := reStorageUsage.FindAllStringSubmatch(text, -1)
	if len(usages) > 0 {
		out.DiskInitialUsage = usages[0][1]
		out.DiskCapacity = usages[0][2]
		lastIdx := len(usages) - 1
		out.DiskFinalUsage = usages[lastIdx][1]
		out.DiskFinalUsageGb = parseSizeToGb(out.DiskFinalUsage)
		if len(usages) >= 3 {
			out.DiskAfterSourceInit = usages[1][1]
			out.DiskAfterSourceInitGb = parseSizeToGb(out.DiskAfterSourceInit)
		} else {
			out.DiskAfterSourceInit = usages[0][1]
			out.DiskAfterSourceInitGb = parseSizeToGb(out.DiskAfterSourceInit)
		}
	}
}

func parseBuildLogTail(tail string, out *parsedGCSBuildTelemetry) {
	if m := reBuildCompleted.FindStringSubmatch(tail); len(m) >= 2 {
		out.CompileDurationSec = parseSoongCompletionDuration(m[1])
	}
	if m := reABFSNinjaEval.FindStringSubmatch(tail); len(m) >= 5 {
		if d, err := time.ParseDuration(m[1]); err == nil {
			out.NinjaEvalSec = d.Seconds()
		}
		hits, _ := strconv.Atoi(m[2])
		misses, _ := strconv.Atoi(m[3])
		ratio, _ := strconv.ParseFloat(m[4], 64)
		out.CacheHits = hits
		out.CacheMisses = misses
		out.CacheHitRatio = ratio
		out.HasRemoteCacheStats = true
		if hits+misses > 0 {
			out.TotalNinjaActions = hits + misses
		}
	}
	if m := reABFSBuildHash.FindStringSubmatch(tail); len(m) >= 2 {
		out.BuildInfoHash = m[1]
	}
	if m := reNinjaFullSteps.FindAllStringSubmatch(tail, -1); len(m) > 0 {
		last := m[len(m)-1]
		if total, err := strconv.Atoi(last[2]); err == nil && total > 0 && out.TotalNinjaActions == 0 {
			out.TotalNinjaActions = total
		}
	}
	if m := reSisoNinjaSteps.FindAllStringSubmatch(tail, -1); len(m) > 0 {
		last := m[len(m)-1]
		if d, err := time.ParseDuration(last[1]); err == nil && out.NinjaEvalSec == 0 {
			out.NinjaEvalSec = d.Seconds()
		}
		if steps, err := strconv.Atoi(last[2]); err == nil && steps > 0 && out.TotalNinjaActions == 0 {
			out.TotalNinjaActions = steps
		}
	}
}

func parseSoongCompletionDuration(raw string) float64 {
	// Examples: "47:34 (mm:ss)", "01:03:01 (hh:mm:ss)", "42 seconds"
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return 0
	}
	first := fields[0]
	if strings.Contains(raw, "second") {
		val, _ := strconv.ParseFloat(first, 64)
		return val
	}
	parts := strings.Split(first, ":")
	switch len(parts) {
	case 2:
		mm, _ := strconv.Atoi(parts[0])
		ss, _ := strconv.Atoi(parts[1])
		return float64(mm*60 + ss)
	case 3:
		hh, _ := strconv.Atoi(parts[0])
		mm, _ := strconv.Atoi(parts[1])
		ss, _ := strconv.Atoi(parts[2])
		return float64(hh*3600 + mm*60 + ss)
	default:
		return 0
	}
}

func parseSizeToGb(raw string) float64 {
	s := strings.TrimSpace(strings.ToUpper(raw))
	if s == "" {
		return 0
	}
	unit := s[len(s)-1]
	numStr := s
	if unit >= 'A' && unit <= 'Z' {
		numStr = s[:len(s)-1]
	}
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case 'T':
		return val * 1024.0
	case 'G':
		return val
	case 'M':
		return val / 1024.0
	case 'K':
		return val / (1024.0 * 1024.0)
	default:
		return val / (1024.0 * 1024.0 * 1024.0)
	}
}

func round1(v float64) float64 {
	return float64(int64(v*10.0+0.5)) / 10.0
}

func round2(v float64) float64 {
	return float64(int64(v*100.0+0.5)) / 100.0
}
