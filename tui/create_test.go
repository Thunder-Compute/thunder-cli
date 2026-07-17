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
		"l40_x1":  {DisplayName: "NVIDIA L40", GpuCount: 1, VcpuOptions: []int{8}, StorageGB: api.StorageRange{Min: 100, Max: 500}},
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

func TestCreateHybridL40PresetUsesDynamicSpecs(t *testing.T) {
	gpu := "L40"
	m := NewCreateModelWithPresets(nil, createTestSpecStore(), &CreatePresets{GPUType: &gpu})

	if m.config.GPUType != "l40" {
		t.Fatalf("expected GPU preset %q to normalize to %q, got %q", gpu, "l40", m.config.GPUType)
	}
	if !m.skippedSteps[stepGPU] {
		t.Fatalf("expected GPU step to be skipped for dynamic preset %q", gpu)
	}
	if m.step != stepCompute {
		t.Fatalf("expected to continue to compute step, got %v", m.step)
	}
}

func TestCreateSnapshotBrowserShowsStatuses(t *testing.T) {
	m := NewCreateModel(nil, createTestSpecStore())
	m.step = stepTemplate
	m.snapshotBrowse = true
	m.snapshotsLoaded = true
	m.snapshots = []api.Snapshot{
		{Name: "usable", MinimumDiskSizeGB: 150, Status: "READY"},
		{Name: "building", MinimumDiskSizeGB: 200, Status: "CREATING"},
		{Name: "broken", MinimumDiskSizeGB: 300, Status: "FAILED"},
	}
	m.cursor = 1

	view := m.View()
	for _, want := range []string{
		"usable (150 GB) — READY",
		"building (200 GB) — CREATING (not ready)",
		"broken (300 GB) — FAILED (unavailable)",
		`Snapshot "building" is CREATING. Only READY snapshots can be used for creation.`,
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected snapshot browser to contain %q, got:\n%s", want, view)
		}
	}
}

func TestCreateSnapshotBrowserRejectsNonReadySelection(t *testing.T) {
	for _, status := range []string{"CREATING", "FAILED", "UNKNOWN"} {
		t.Run(status, func(t *testing.T) {
			m := NewCreateModel(nil, createTestSpecStore())
			m.step = stepTemplate
			m.snapshotBrowse = true
			m.snapshotsLoaded = true
			m.snapshots = []api.Snapshot{{Name: "not-ready", MinimumDiskSizeGB: 200, Status: status}}

			model, _ := m.handleEnter()
			got := model.(createModel)
			if got.step != stepTemplate || !got.snapshotBrowse {
				t.Fatalf("expected %s snapshot to remain in snapshot browser, got step=%v browse=%t", status, got.step, got.snapshotBrowse)
			}
			if got.config.Template != "" || got.selectedSnapshot != nil {
				t.Fatalf("expected %s snapshot not to be selected", status)
			}
		})
	}
}

func TestCreateSnapshotBrowserAcceptsReadySelection(t *testing.T) {
	m := NewCreateModel(nil, createTestSpecStore())
	m.step = stepTemplate
	m.snapshotBrowse = true
	m.snapshotsLoaded = true
	m.snapshots = []api.Snapshot{{Name: "usable", MinimumDiskSizeGB: 200, Status: "READY"}}

	model, _ := m.handleEnter()
	got := model.(createModel)
	if got.step != stepDiskSize {
		t.Fatalf("expected READY snapshot to advance to disk step, got %v", got.step)
	}
	if got.config.Template != "usable" || got.selectedSnapshot == nil {
		t.Fatalf("expected READY snapshot to be selected, got template=%q snapshot=%v", got.config.Template, got.selectedSnapshot)
	}
}

func TestCreateSnapshotPresetRejectsNonReadySnapshot(t *testing.T) {
	name := "building"
	m := NewCreateModel(nil, createTestSpecStore())
	m.step = stepTemplate
	m.presets = &CreatePresets{Template: &name}
	m.templatesLoaded = true
	m.snapshotsLoaded = true
	m.snapshots = []api.Snapshot{
		{Name: "usable", MinimumDiskSizeGB: 100, Status: "READY"},
		{Name: name, MinimumDiskSizeGB: 200, Status: "CREATING"},
	}

	m.trySkipTemplate()
	if m.step != stepTemplate || !m.snapshotBrowse || m.cursor != 1 {
		t.Fatalf("expected non-ready preset to open highlighted snapshot browser, got step=%v browse=%t cursor=%d", m.step, m.snapshotBrowse, m.cursor)
	}
	if m.config.Template != "" || m.selectedSnapshot != nil || m.skippedSteps[stepTemplate] {
		t.Fatal("expected non-ready snapshot preset not to be applied")
	}
}

func TestCreateSnapshotPresetAcceptsReadySnapshot(t *testing.T) {
	name := "usable"
	m := NewCreateModel(nil, createTestSpecStore())
	m.step = stepTemplate
	m.presets = &CreatePresets{Template: &name}
	m.templatesLoaded = true
	m.snapshotsLoaded = true
	m.snapshots = []api.Snapshot{{Name: name, MinimumDiskSizeGB: 200, Status: "READY"}}

	m.trySkipTemplate()
	if m.step != stepDiskSize || !m.skippedSteps[stepTemplate] {
		t.Fatalf("expected READY snapshot preset to advance to disk step, got step=%v skipped=%t", m.step, m.skippedSteps[stepTemplate])
	}
	if m.config.Template != name || m.selectedSnapshot == nil {
		t.Fatalf("expected READY snapshot preset to be applied, got template=%q snapshot=%v", m.config.Template, m.selectedSnapshot)
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
	// SpecStore via computePreviewPrice; it keeps the cost label empty instead.
	model, _ = cm.Update(createPricingMsg{rates: map[string]float64{}})
	cm = model.(createModel)
	if cm.pricing == nil {
		t.Fatal("expected pricing to be set after createPricingMsg")
	}
	if view := cm.View(); !strings.Contains(view, "Estimated cost:") || strings.Contains(view, "calculating") {
		t.Fatalf("expected an empty estimated cost line before specs load, got:\n%s", view)
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
