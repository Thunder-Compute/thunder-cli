package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Thunder-Compute/thunder-cli/utils"
)

// TestCreateModelSurvivesInputBeforeSpecsLoad guards the flagless interactive
// flow, where GPU specs load asynchronously after the TUI opens. The user can
// reach (and interact with) the GPU step before specs arrive, so the model must
// render and handle input without dereferencing a nil SpecStore.
func TestCreateModelSurvivesInputBeforeSpecsLoad(t *testing.T) {
	m := NewCreateModel(nil, nil) // nil specs => asynchronous-load path
	if m.specsLoaded {
		t.Fatal("expected specsLoaded=false when constructed without specs")
	}

	// Select a mode: advances to the GPU step and runs initStep with nil specs.
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	cm := model.(createModel)
	if cm.step != stepGPU {
		t.Fatalf("expected stepGPU after mode selection, got %v", cm.step)
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

// TestCreateHybridSkipsModeStepForDevelopmentAlias guards the hybrid (flag +
// TUI) flow against the "development" user-facing alias. The preset value is the
// raw flag string, so the skip logic must normalize it the same way
// validateCreateConfig does; otherwise --mode development silently fails to
// pre-fill the mode step and forces the user to re-select it interactively.
func TestCreateHybridSkipsModeStepForDevelopmentAlias(t *testing.T) {
	for _, input := range []string{"development", "Development", "prototyping", "production"} {
		t.Run(input, func(t *testing.T) {
			in := input
			m := NewCreateModelWithPresets(nil, nil, &CreatePresets{Mode: &in})

			if !m.skippedSteps[stepMode] {
				t.Fatalf("expected stepMode to be auto-skipped for --mode %q", input)
			}
			if m.step == stepMode {
				t.Fatalf("expected to advance past stepMode for --mode %q", input)
			}

			want := utils.NormalizeModeInput(input)
			if m.config.Mode != want {
				t.Fatalf("expected wire mode %q for --mode %q, got %q", want, input, m.config.Mode)
			}
		})
	}
}
