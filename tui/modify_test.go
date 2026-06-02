package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

func TestModifyModelSingleCountProductionGPUInitializesNumGPUs(t *testing.T) {
	specs := utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping": {
			GpuCount:           1,
			Mode:               "prototyping",
			VcpuOptions:        []int{4, 8},
			RamPerVCPUGiB:      8,
			StorageGB:          api.StorageRange{Min: 100, Max: 300},
			EphemeralStorageGB: api.StorageRange{Min: 0, Max: 2000},
		},
		"a6000_x1_production": {
			GpuCount:           1,
			Mode:               "production",
			VcpuOptions:        []int{18},
			RamPerVCPUGiB:      5,
			StorageGB:          api.StorageRange{Min: 100, Max: 1000},
			EphemeralStorageGB: api.StorageRange{Min: 0, Max: 2000},
		},
	})
	instance := &api.Instance{
		ID:       "0",
		Name:     "rqrljt9j",
		Status:   "RUNNING",
		Mode:     "prototyping",
		GPUType:  "a6000",
		NumGPUs:  "1",
		CPUCores: "8",
		Storage:  100,
	}

	model := NewModifyModel(nil, instance, specs).(modifyModel)

	// Select production mode.
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(modifyModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	if model.step != modifyStepGPU {
		t.Fatalf("expected GPU step after selecting production, got %v", model.step)
	}

	// Keep A6000 and enter compute. Production A6000 only has one GPU-count
	// option, so the TUI should initialize NumGPUs instead of leaving it as 0.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
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
		"a6000_x1_production": 1.25,
	}})
	model = updated.(modifyModel)
	if view := model.View(); !strings.Contains(view, "18 vCPUs") {
		t.Fatalf("expected production vCPU option to render, got:\n%s", view)
	}
}

func TestModifyModelChangingGPUClearsStaleGPUCount(t *testing.T) {
	specs := utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping": {GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8}},
		"a6000_x4_prototyping": {GpuCount: 4, Mode: "prototyping", VcpuOptions: []int{16, 24}},
		"h100_x1_prototyping":  {GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{8, 12}},
		"h100_x2_prototyping":  {GpuCount: 2, Mode: "prototyping", VcpuOptions: []int{16, 24}},
	})
	instance := &api.Instance{
		ID:       "0",
		Name:     "rqrljt9j",
		Status:   "RUNNING",
		Mode:     "prototyping",
		GPUType:  "a6000",
		NumGPUs:  "1",
		CPUCores: "8",
		Storage:  100,
	}

	model := NewModifyModel(nil, instance, specs).(modifyModel)

	// Keep prototyping and A6000, then select 4 GPUs.
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	if !model.gpuCountPhase {
		t.Fatal("expected A6000 to show a GPU-count phase")
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(modifyModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	if model.config.NumGPUs != 4 {
		t.Fatalf("expected stale setup to select 4 GPUs, got %d", model.config.NumGPUs)
	}

	// Back up to GPU selection and choose H100. The old A6000 x4 count must not
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
		"a6000_x1_prototyping": {GpuCount: 1, Mode: "prototyping", VcpuOptions: []int{4, 8}},
		"a6000_x4_prototyping": {GpuCount: 4, Mode: "prototyping", VcpuOptions: []int{16, 24}},
	}, map[string]string{
		"a6000_x1_prototyping": "available",
		"a6000_x4_prototyping": "unavailable",
	})
	instance := &api.Instance{
		ID:       "0",
		Name:     "rqrljt9j",
		Status:   "RUNNING",
		Mode:     "prototyping",
		GPUType:  "a6000",
		NumGPUs:  "1",
		CPUCores: "8",
		Storage:  100,
	}

	model := NewModifyModel(nil, instance, specs).(modifyModel)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(modifyModel)
	if !model.gpuCountPhase {
		t.Fatal("expected A6000 to show a GPU-count phase")
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

func TestModifyModelPresetsDoNotSkipEphemeralDiskShrink(t *testing.T) {
	specs := utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"a6000_x1_prototyping": {
			GpuCount:           1,
			Mode:               "prototyping",
			VcpuOptions:        []int{4, 8},
			RamPerVCPUGiB:      8,
			StorageGB:          api.StorageRange{Min: 100, Max: 300},
			EphemeralStorageGB: api.StorageRange{Min: 0, Max: 2000},
		},
	})
	instance := &api.Instance{
		ID:              "0",
		Name:            "rqrljt9j",
		Status:          "RUNNING",
		Mode:            "prototyping",
		GPUType:         "a6000",
		NumGPUs:         "1",
		CPUCores:        "8",
		Storage:         100,
		EphemeralDiskGB: 200,
	}
	mode := "prototyping"
	gpu := "a6000"
	vcpus := 8
	diskGB := 100
	ephemeralGB := 100
	presets := &ModifyPresets{
		Mode:            &mode,
		GPUType:         &gpu,
		VCPUs:           &vcpus,
		DiskSizeGB:      &diskGB,
		EphemeralDiskGB: &ephemeralGB,
	}

	model := NewModifyModelWithPresets(nil, instance, specs, presets)

	if model.step != modifyStepEphemeralDiskSize {
		t.Fatalf("expected invalid ephemeral preset to stop at ephemeral step, got %v", model.step)
	}
	if model.skippedSteps[modifyStepEphemeralDiskSize] {
		t.Fatal("ephemeral shrink preset should not be marked skipped")
	}
	if model.config.EphemeralDiskChanged {
		t.Fatal("ephemeral shrink preset should not be accepted into config")
	}
}
