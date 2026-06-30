package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

func TestModifyModelSingleCountGPUInitializesNumGPUs(t *testing.T) {
	specs := utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1": {
			GpuCount:      1,
			VcpuOptions:   []int{18},
			RamPerVCPUGiB: 5,
			StorageGB:     api.StorageRange{Min: 100, Max: 1000},
		},
	})
	instance := &api.Instance{
		ID:       "0",
		Name:     "rqrljt9j",
		Status:   "RUNNING",
		GPUType:  "a6000",
		NumGPUs:  "1",
		CPUCores: "8",
		Storage:  100,
	}

	model := NewModifyModel(nil, instance, specs).(modifyModel)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	// Keep A6000 and enter compute. A6000 only has one GPU-count
	// option, so the TUI should initialize NumGPUs instead of leaving it as 0.
	if model.step != modifyStepCompute {
		t.Fatalf("expected compute step after selecting GPU, got %v", model.step)
	}
	if model.gpuCountPhase {
		t.Fatal("single-count production GPU should not show a GPU-count phase")
	}
	if model.config.NumGPUs != 1 {
		t.Fatalf("expected NumGPUs to initialize to 1, got %d", model.config.NumGPUs)
	}

	updated, _ = model.Update(modifyPricingMsg{rates: map[string]float64{
		"a6000_x1": 1.25,
	}})
	model = updated.(modifyModel)
	if view := model.View(); !strings.Contains(view, "18 vCPUs") {
		t.Fatalf("expected vCPU option to render, got:\n%s", view)
	}
}

func TestModifyModelChangingGPUClearsStaleGPUCount(t *testing.T) {
	specs := utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a100xl_x1": {GpuCount: 1, VcpuOptions: []int{4, 8}},
		"a100xl_x4": {GpuCount: 4, VcpuOptions: []int{16, 24}},
		"h100_x1":   {GpuCount: 1, VcpuOptions: []int{8, 12}},
		"h100_x2":   {GpuCount: 2, VcpuOptions: []int{16, 24}},
	})
	instance := &api.Instance{
		ID:       "0",
		Name:     "rqrljt9j",
		Status:   "RUNNING",
		GPUType:  "a100xl",
		NumGPUs:  "1",
		CPUCores: "8",
		Storage:  100,
	}

	model := NewModifyModel(nil, instance, specs).(modifyModel)

	// Keep A100, then select 4 GPUs.
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	if !model.gpuCountPhase {
		t.Fatal("expected A100 to show a GPU-count phase")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(modifyModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	if model.config.NumGPUs != 4 {
		t.Fatalf("expected stale setup to select 4 GPUs, got %d", model.config.NumGPUs)
	}

	// Back up to GPU selection and choose H100. The old A100 x4 count must not
	// be reused for H100, whose valid counts are 1 and 2.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(modifyModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(modifyModel)
	if model.step != modifyStepGPU {
		t.Fatalf("expected GPU step after backing up, got %v", model.step)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(modifyModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)

	if model.step != modifyStepCompute {
		t.Fatalf("expected compute step after selecting H100, got %v", model.step)
	}
	if !model.gpuCountPhase {
		t.Fatal("expected H100 to start at GPU-count selection after stale count reset")
	}
	if model.config.NumGPUs != 0 {
		t.Fatalf("expected stale NumGPUs to be cleared before H100 count selection, got %d", model.config.NumGPUs)
	}
}

func TestModifyModelUnavailableGPUCountCannotBeSelected(t *testing.T) {
	specs := utils.NewSpecStoreWithAvailability(map[string]api.GpuSpecConfig{
		"a100xl_x1": {GpuCount: 1, VcpuOptions: []int{4, 8}},
		"a100xl_x4": {GpuCount: 4, VcpuOptions: []int{16, 24}},
	}, map[string]string{
		"a100xl_x1": "available",
		"a100xl_x4": "unavailable",
	})
	instance := &api.Instance{
		ID:       "0",
		Name:     "rqrljt9j",
		Status:   "RUNNING",
		GPUType:  "a100xl",
		NumGPUs:  "1",
		CPUCores: "8",
		Storage:  100,
	}

	model := NewModifyModel(nil, instance, specs).(modifyModel)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	if !model.gpuCountPhase {
		t.Fatal("expected A100 to show a GPU-count phase")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(modifyModel)
	if view := model.View(); !strings.Contains(view, "4 GPU(s) (unavailable)") {
		t.Fatalf("expected unavailable GPU count to render, got:\n%s", view)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	if !model.gpuCountPhase {
		t.Fatal("expected to remain in GPU-count phase after selecting unavailable count")
	}
	if model.config.NumGPUs != 0 {
		t.Fatalf("expected unavailable count not to be selected, got %d", model.config.NumGPUs)
	}
}
