package toolsidecar

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type SkillImageMapping struct {
	SkillName string
	Image     string
}

type SkillRouterConfig struct {
	Provisioner     *Provisioner
	ImageMappings   []SkillImageMapping
	IdleTimeout     time.Duration
	MaxPerWorkflow  int
}

type SkillRouter struct {
	provisioner    *Provisioner
	imageMap       map[string]string
	idleTimeout    time.Duration
	maxPerWorkflow int

	mu             sync.RWMutex
	active         map[string]*TrackedSidecar
	workflowCounts map[string]int
}

type TrackedSidecar struct {
	Sidecar     *ToolSidecar
	WorkflowID  string
	LastActive  time.Time
	StopTimer   *time.Timer
}

func NewSkillRouter(cfg SkillRouterConfig) (*SkillRouter, error) {
	if cfg.Provisioner == nil {
		return nil, fmt.Errorf("skill_router: provisioner is required")
	}

	imageMap := make(map[string]string)
	for _, m := range cfg.ImageMappings {
		imageMap[m.SkillName] = m.Image
	}

	idleTimeout := cfg.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = 15 * time.Minute
	}

	maxPerWorkflow := cfg.MaxPerWorkflow
	if maxPerWorkflow == 0 {
		maxPerWorkflow = 3
	}

	return &SkillRouter{
		provisioner:    cfg.Provisioner,
		imageMap:       imageMap,
		idleTimeout:    idleTimeout,
		maxPerWorkflow: maxPerWorkflow,
		active:         make(map[string]*TrackedSidecar),
		workflowCounts: make(map[string]int),
	}, nil
}

func (r *SkillRouter) Spawn(ctx context.Context, skillName, sessionID, workflowID string) (*ToolSidecar, error) {
	if skillName == "" {
		return nil, fmt.Errorf("skill_router: skill_name is required")
	}
	if sessionID == "" {
		return nil, fmt.Errorf("skill_router: session_id is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.maxPerWorkflow > 0 && workflowID != "" {
		if r.workflowCounts[workflowID] >= r.maxPerWorkflow {
			return nil, fmt.Errorf("skill_router: workflow %s has reached max concurrent sidecars (%d)", workflowID, r.maxPerWorkflow)
		}
	}

	image, ok := r.imageMap[skillName]
	if !ok {
		image = ""
	}

	origImage := r.provisioner.defaultImage
	if image != "" {
		r.provisioner.defaultImage = image
	}

	sidecar, err := r.provisioner.SpawnToolSidecar(ctx, skillName, sessionID)

	if image != "" {
		r.provisioner.defaultImage = origImage
	}

	if err != nil {
		return nil, err
	}

	tracked := &TrackedSidecar{
		Sidecar:    sidecar,
		WorkflowID: workflowID,
		LastActive: time.Now(),
	}

	if r.idleTimeout > 0 {
		containerID := sidecar.ID
		tracked.StopTimer = time.AfterFunc(r.idleTimeout, func() {
			r.stopIdle(containerID)
		})
	}

	r.active[sidecar.ID] = tracked
	if workflowID != "" {
		r.workflowCounts[workflowID]++
	}

	return sidecar, nil
}

func (r *SkillRouter) List() []*ToolSidecar {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ToolSidecar, 0, len(r.active))
	for _, tracked := range r.active {
		result = append(result, tracked.Sidecar)
	}
	return result
}

func (r *SkillRouter) Stop(ctx context.Context, containerID string) error {
	r.mu.Lock()
	tracked, ok := r.active[containerID]
	if ok {
		if tracked.StopTimer != nil {
			tracked.StopTimer.Stop()
		}
		delete(r.active, containerID)
		if tracked.WorkflowID != "" {
			r.workflowCounts[tracked.WorkflowID]--
			if r.workflowCounts[tracked.WorkflowID] <= 0 {
				delete(r.workflowCounts, tracked.WorkflowID)
			}
		}
	}
	r.mu.Unlock()

	if !ok {
		return fmt.Errorf("skill_router: sidecar %s not found", containerID)
	}

	return r.provisioner.StopToolSidecar(ctx, containerID)
}

func (r *SkillRouter) stopIdle(containerID string) {
	r.mu.Lock()
	tracked, ok := r.active[containerID]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.active, containerID)
	if tracked.WorkflowID != "" {
		r.workflowCounts[tracked.WorkflowID]--
		if r.workflowCounts[tracked.WorkflowID] <= 0 {
			delete(r.workflowCounts, tracked.WorkflowID)
		}
	}
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = r.provisioner.StopToolSidecar(ctx, containerID)
}

func (r *SkillRouter) GetImageForSkill(skillName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	img, ok := r.imageMap[skillName]
	return img, ok
}

func (r *SkillRouter) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.active)
}

func (r *SkillRouter) WorkflowCount(workflowID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.workflowCounts[workflowID]
}
