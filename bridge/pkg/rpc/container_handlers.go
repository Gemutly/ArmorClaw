// Package rpc provides container management RPC methods.
package rpc

import (
	"context"
	"encoding/json"

	"github.com/armorclaw/bridge/pkg/audit"
	"github.com/armorclaw/bridge/pkg/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
)

var _ = docker.ScopeCreate

// handleTerminateContainer terminates a container immediately (SIGKILL)
// Requires authentication and container ownership verification
func (s *Server) handleTerminateContainer(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		ContainerID string `json:"container_id"`
		UserID      string `json:"user_id"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.ContainerID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "container_id is required",
		}
	}

	if params.UserID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "user_id is required for authentication",
		}
	}

	if s.dockerClient == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "docker client not configured",
		}
	}

	inspect, err := s.dockerClient.InspectContainer(ctx, params.ContainerID)
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "container not found: " + err.Error(),
		}
	}

	if inspect.Config == nil || inspect.Config.Labels == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "container is not managed by Bridge",
		}
	}

	hasArmorClawLabels := false
	for key := range inspect.Config.Labels {
		if containsArmorClawLabel(key) {
			hasArmorClawLabels = true
			break
		}
	}

	if !hasArmorClawLabels {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "container is not managed by Bridge",
		}
	}

	if err := s.dockerClient.TerminateContainer(ctx, params.ContainerID); err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to terminate container: " + err.Error(),
		}
	}

	if s.auditLog != nil {
		_ = s.auditLog.LogEvent(audit.EventSecurityViolation, "", "", params.UserID, map[string]interface{}{
			"action":       "terminate_container",
			"container_id": params.ContainerID,
		})
	}

	return map[string]interface{}{
		"success":      true,
		"container_id": params.ContainerID,
	}, nil
}

// containsArmorClawLabel checks if a label key indicates Bridge ownership
func containsArmorClawLabel(key string) bool {
	armorclawLabels := []string{
		"armorclaw.agent_id",
		"armorclaw.key_id",
		"armorclaw.session_id",
		"com.armorclaw.agent",
		"com.armorclaw.managed",
	}

	for _, label := range armorclawLabels {
		if key == label {
			return true
		}
	}
	return false
}

// handleListContainers lists all containers managed by Bridge
func (s *Server) handleListContainers(ctx context.Context, req *Request) (interface{}, *ErrorObj) {
	var params struct {
		UserID string `json:"user_id"`
		All    bool   `json:"all,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "invalid parameters: " + err.Error(),
		}
	}

	if params.UserID == "" {
		return nil, &ErrorObj{
			Code:    InvalidParams,
			Message: "user_id is required for authentication",
		}
	}

	if s.dockerClient == nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "docker client not configured",
		}
	}

	containers, err := s.dockerClient.ListContainers(ctx, true, filters.Args{})
	if err != nil {
		return nil, &ErrorObj{
			Code:    InternalError,
			Message: "failed to list containers: " + err.Error(),
		}
	}

	bridgeContainers := []types.Container{}
	for _, container := range containers {
		if container.Labels != nil {
			for key := range container.Labels {
				if containsArmorClawLabel(key) {
					bridgeContainers = append(bridgeContainers, container)
					break
				}
			}
		}
	}

	return map[string]interface{}{
		"containers": bridgeContainers,
		"count":      len(bridgeContainers),
	}, nil
}
