package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

func createTestSpecStore() *utils.SpecStore {
	return utils.NewSpecStore(map[string]api.GpuSpecConfig{
		"h100_x1": {GpuCount: 1, VcpuOptions: []int{15}, StorageGB: api.StorageRange{Min: 100, Max: 500}},
		"h100_x2": {GpuCount: 2, VcpuOptions: []int{30}, StorageGB: api.StorageRange{Min: 100, Max: 1000}},
		"h100_x4": {GpuCount: 4, VcpuOptions: []int{60}, StorageGB: api.StorageRange{Min: 100, Max: 2000}},
	})
}

func TestCreateDefaultDiskSizeUsesFreeStoragePerGPU(t *testing.T) {
	m := NewCreateModel(nil, createTestSpecStore())
	m.config.GPUType = "h100"
	m.config.NumGPUs = 4

	if got := m.defaultDiskSizeGB(); got != 400 {
		t.Fatalf("expected default disk size 400GB for 4 GPUs, got %dGB", got)
	}
}

func TestCreateDefaultDiskSizeRespectsSnapshotMinimum(t *testing.T) {
	m := NewCreateModel(nil, createTestSpecStore())
	m.config.GPUType = "h100"
	m.config.NumGPUs = 4
	m.selectedSnapshot = &api.Snapshot{MinimumDiskSizeGB: 550}

	if got := m.defaultDiskSizeGB(); got != 550 {
		t.Fatalf("expected snapshot minimum disk size 550GB, got %dGB", got)
	}
}

func TestCreateDiskInputSeedsFromGPUCount(t *testing.T) {
	m := NewCreateModel(nil, createTestSpecStore())
	m.config.GPUType = "h100"
	m.config.NumGPUs = 4
	m.setDefaultDiskSize()
	m.step = stepDiskSize
	m.initStep()

	if got := m.config.DiskSizeGB; got != 400 {
		t.Fatalf("expected config disk size 400GB, got %dGB", got)
	}
	if got := m.diskInput.Value(); got != "400" {
		t.Fatalf("expected disk input value 400, got %q", got)
	}
}

// TestCreateModelSurvivesInputBeforeSpecsLoad guards the flagless interactive
// flow, where GPU specs load asynchronously after the TUI opens. The user can
// reach (and interact with) the GPU step before specs arrive, so the model must
// render and handle input without dereferencing a nil SpecStore.
func TestCreateModelSurvivesInputBeforeSpecsLoad(t *testing.T) {
	m := NewCreateModel(nil, nil) // nil specs => asynchronous-load path
	if m.specsLoaded {
		t.Fatal("expected specsLoaded=false when constructed without specs")
	}

	// The flow starts at GPU selection and should remain safe before specs load.
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	cm := model.(createModel)
	if cm.step != stepGPU {
		t.Fatalf("expected stepGPU before specs load, got %v", cm.step)
	}

	// Pricing typically arrives before specs in the async flow. Rendering the
	// GPU step with pricing present but specs still nil must not deref the nil
	// SpecStore via computePreviewPrice — instead it shows a calculating spinner.
	model, _ = cm.Update(createPricingMsg{rates: map[string]float64{}})
	cm = model.(createModel)
	if cm.pricing == nil {
		t.Fatal("expected pricing to be set after createPricingMsg")
	}
	if view := cm.View(); !strings.Contains(view, "calculating") {
		t.Fatalf("expected a calculating indicator before specs load, got:\n%s", view)
	}

	// Navigation must also be safe while specs are still loading.
	model, _ = cm.Update(tea.KeyMsg{Type: tea.KeyDown})
	cm = model.(createModel)
	model, _ = cm.Update(tea.KeyMsg{Type: tea.KeyEnter}) // selection ignored pre-load
	cm = model.(createModel)
	if cm.step != stepGPU {
		t.Fatalf("expected to remain on stepGPU until specs load, got %v", cm.step)
	}
	if cm.specsLoaded {
		t.Fatal("specs should not be marked loaded before createSpecsMsg")
	}

	// Specs arrive: the model marks them loaded and re-inits the current step.
	model, _ = cm.Update(createSpecsMsg{specs: utils.NewSpecStoreWithAvailability(nil, nil)})
	cm = model.(createModel)
	if !cm.specsLoaded {
		t.Fatal("expected specsLoaded=true after createSpecsMsg")
	}
	if view := cm.View(); !strings.Contains(view, "Estimated cost") {
		t.Fatalf("expected an estimated cost line after specs load, got:\n%s", view)
	}
}
