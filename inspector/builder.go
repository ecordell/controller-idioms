// Package inspector provides XState machine building through stage instrumentation
package inspector

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/authzed/controller-idioms/state"
)

// XStateBuilder builds XState machine definitions by instrumenting state.NewStage functions
type XStateBuilder struct {
	mu           sync.RWMutex
	machineID    string
	nodes        map[string]*XStateNode
	transitions  map[string][]string // from -> []to
	stateCounter int64
	rootStateID  string
}

// NewXStateBuilder creates a new XState builder
func NewXStateBuilder(machineID string) *XStateBuilder {
	return &XStateBuilder{
		machineID:   machineID,
		nodes:       make(map[string]*XStateNode),
		transitions: make(map[string][]string),
	}
}

// InstrumentSequence instruments a Sequence stage and records its XState structure
func (b *XStateBuilder) InstrumentSequence(stages ...state.NewStage) state.NewStage {
	sequenceID := b.generateStateID("sequence")

	// Create compound state for sequence
	node := &XStateNode{
		Type:    "compound",
		Initial: "",
		States:  make(map[string]*XStateNode),
		On:      make(map[string]*XStateTransition),
		Meta: map[string]interface{}{
			"type":       "sequence",
			"stageCount": len(stages),
		},
	}

	// Instrument each stage in the sequence
	instrumentedStages := make([]state.NewStage, len(stages))

	for i, stage := range stages {
		stageID := b.generateStateID(fmt.Sprintf("seq_step_%d", i))
		instrumentedStages[i] = b.InstrumentStage(stageID, stage)

		// Add to sequence's compound state
		stepNode := &XStateNode{
			Type: "atomic",
			Meta: map[string]interface{}{
				"sequenceIndex": i,
			},
		}

		// Add automatic transition to next step
		if i < len(stages)-1 {
			nextStageID := b.generateStateID(fmt.Sprintf("seq_step_%d", i+1))
			stepNode.Always = []*XStateTransition{{
				Target: nextStageID,
			}}
			b.addTransition(stageID, nextStageID)
		}

		node.States[stageID] = stepNode

		if i == 0 {
			node.Initial = stageID
		}
	}

	b.addNode(sequenceID, node)

	return func() state.Stage {
		return state.Sequence(instrumentedStages...).Stage()
	}
}

// InstrumentParallel instruments a Parallel stage and records its XState structure
func (b *XStateBuilder) InstrumentParallel(stages ...state.NewStage) state.NewStage {
	parallelID := b.generateStateID("parallel")

	// Create parallel state
	node := &XStateNode{
		Type:   "parallel",
		States: make(map[string]*XStateNode),
		Meta: map[string]interface{}{
			"type":       "parallel",
			"stageCount": len(stages),
		},
	}

	// Instrument each parallel stage
	instrumentedStages := make([]state.NewStage, len(stages))
	for i, stage := range stages {
		stageID := b.generateStateID(fmt.Sprintf("par_branch_%d", i))
		instrumentedStages[i] = b.InstrumentStage(stageID, stage)

		// Add parallel branch
		node.States[stageID] = &XStateNode{
			Type: "atomic",
			Meta: map[string]interface{}{
				"parallelIndex": i,
			},
		}
	}

	b.addNode(parallelID, node)

	return func() state.Stage {
		return state.Parallel(instrumentedStages...).Stage()
	}
}

// InstrumentDecision instruments a Decision stage and records its XState structure
func (b *XStateBuilder) InstrumentDecision(
	predicate func(context.Context) bool,
	trueStage, falseStage state.NewStage,
) state.NewStage {
	decisionID := b.generateStateID("decision")

	trueID := b.generateStateID("decision_true")
	falseID := b.generateStateID("decision_false")

	// Create decision state with conditional transitions
	node := &XStateNode{
		Type: "atomic",
		On: map[string]*XStateTransition{
			"EVALUATE": {
				Target: "", // Will be determined at runtime
				Guard: XStateGuard{
					Type: "evaluateCondition",
					Meta: map[string]interface{}{
						"trueTarget":  trueID,
						"falseTarget": falseID,
					},
				},
			},
		},
		Meta: map[string]interface{}{
			"type": "decision",
		},
	}

	b.addNode(decisionID, node)
	b.addTransition(decisionID, trueID)
	b.addTransition(decisionID, falseID)

	// Instrument the branch stages
	instrumentedTrue := b.InstrumentStage(trueID, trueStage)
	instrumentedFalse := b.InstrumentStage(falseID, falseStage)

	return func() state.Stage {
		return state.Decision(predicate, instrumentedTrue, instrumentedFalse).Stage()
	}
}

// InstrumentEnum instruments an Enum/Switch stage and records its XState structure
func (b *XStateBuilder) InstrumentEnum(
	selector func(context.Context) string,
	cases map[string]state.NewStage,
	defaultStage state.NewStage,
) state.NewStage {
	enumID := b.generateStateID("enum")

	// Create enum state with multiple transitions
	node := &XStateNode{
		Type: "atomic",
		On:   make(map[string]*XStateTransition),
		Meta: map[string]interface{}{
			"type":      "enum",
			"caseCount": len(cases),
		},
	}

	// Instrument each case
	instrumentedCases := make(map[string]state.NewStage)
	for caseValue, caseStage := range cases {
		caseID := b.generateStateID(fmt.Sprintf("enum_case_%s", caseValue))
		instrumentedCases[caseValue] = b.InstrumentStage(caseID, caseStage)

		// Add transition for this case
		node.On[fmt.Sprintf("CASE_%s", caseValue)] = &XStateTransition{
			Target: caseID,
		}

		b.addTransition(enumID, caseID)
	}

	// Instrument default case
	var instrumentedDefault state.NewStage
	if defaultStage != nil {
		defaultID := b.generateStateID("enum_default")
		instrumentedDefault = b.InstrumentStage(defaultID, defaultStage)

		node.On["DEFAULT"] = &XStateTransition{
			Target: defaultID,
		}

		b.addTransition(enumID, defaultID)
	}

	b.addNode(enumID, node)

	return func() state.Stage {
		return state.Enum(selector, instrumentedCases, instrumentedDefault).Stage()
	}
}

// InstrumentAction instruments a simple Action stage
func (b *XStateBuilder) InstrumentAction(action func(context.Context)) state.NewStage {
	actionID := b.generateStateID("action")

	node := &XStateNode{
		Type: "atomic",
		Entry: []XStateAction{{
			Type: "executeAction",
			Meta: map[string]interface{}{
				"actionID": actionID,
			},
		}},
		Meta: map[string]interface{}{
			"type": "action",
		},
	}

	b.addNode(actionID, node)

	return func() state.Stage {
		return state.Action(action).Stage()
	}
}

// InstrumentStage instruments a generic stage with a given ID
func (b *XStateBuilder) InstrumentStage(stageID string, stage state.NewStage) state.NewStage {
	// Create a generic node for unknown stage types
	node := &XStateNode{
		Type: "atomic",
		Entry: []XStateAction{{
			Type: "executeStage",
			Meta: map[string]interface{}{
				"stageID": stageID,
			},
		}},
		Meta: map[string]interface{}{
			"type": "generic",
		},
	}

	b.addNode(stageID, node)

	return func() state.Stage {
		// Wrap the original stage with instrumentation
		return state.StageFunc(func(ctx context.Context) state.Stage {
			// Execute the original stage
			originalStage := stage()
			if originalStage == nil {
				return nil
			}

			// Execute and capture transitions
			result := originalStage.Next(ctx)

			// Record transition if there's a result
			if result != nil {
				nextID := b.generateStateID("unknown_next")
				b.addTransition(stageID, nextID)
			}

			return result
		})
	}
}

// BuildMachine creates the final XState machine definition
func (b *XStateBuilder) BuildMachine() *XStateMachine {
	b.mu.RLock()
	defer b.mu.RUnlock()

	machine := &XStateMachine{
		ID:      b.machineID,
		Initial: b.rootStateID,
		States:  make(map[string]*XStateNode),
		Context: make(map[string]interface{}),
		Version: "5",
	}

	// Copy all nodes
	for id, node := range b.nodes {
		machine.States[id] = node
	}

	// If no root state was set, try to find one
	if b.rootStateID == "" && len(b.nodes) > 0 {
		// Use the first node as root
		for id := range b.nodes {
			machine.Initial = id
			break
		}
	}

	return machine
}

// SetRootState sets the root state ID for the machine
func (b *XStateBuilder) SetRootState(stateID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rootStateID = stateID
}

// generateStateID generates a unique state ID
func (b *XStateBuilder) generateStateID(prefix string) string {
	counter := atomic.AddInt64(&b.stateCounter, 1)
	return fmt.Sprintf("%s_%d", prefix, counter)
}

// addNode adds a node to the builder
func (b *XStateBuilder) addNode(id string, node *XStateNode) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nodes[id] = node
}

// addTransition records a transition between states
func (b *XStateBuilder) addTransition(from, to string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.transitions[from] == nil {
		b.transitions[from] = make([]string, 0)
	}

	// Avoid duplicate transitions
	for _, existing := range b.transitions[from] {
		if existing == to {
			return
		}
	}

	b.transitions[from] = append(b.transitions[from], to)
}

// GetTransitions returns all recorded transitions
func (b *XStateBuilder) GetTransitions() map[string][]string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make(map[string][]string)
	for from, tos := range b.transitions {
		result[from] = make([]string, len(tos))
		copy(result[from], tos)
	}
	return result
}

// InstrumentedBuilder provides a fluent interface for building instrumented state machines
type InstrumentedBuilder struct {
	builder *XStateBuilder
}

// NewInstrumentedBuilder creates a new instrumented builder
func NewInstrumentedBuilder(machineID string) *InstrumentedBuilder {
	return &InstrumentedBuilder{
		builder: NewXStateBuilder(machineID),
	}
}

// Sequence creates an instrumented sequence
func (ib *InstrumentedBuilder) Sequence(stages ...state.NewStage) state.NewStage {
	return ib.builder.InstrumentSequence(stages...)
}

// Parallel creates an instrumented parallel stage
func (ib *InstrumentedBuilder) Parallel(stages ...state.NewStage) state.NewStage {
	return ib.builder.InstrumentParallel(stages...)
}

// Decision creates an instrumented decision stage
func (ib *InstrumentedBuilder) Decision(
	predicate func(context.Context) bool,
	trueStage, falseStage state.NewStage,
) state.NewStage {
	return ib.builder.InstrumentDecision(predicate, trueStage, falseStage)
}

// Enum creates an instrumented enum stage
func (ib *InstrumentedBuilder) Enum(
	selector func(context.Context) string,
	cases map[string]state.NewStage,
	defaultStage state.NewStage,
) state.NewStage {
	return ib.builder.InstrumentEnum(selector, cases, defaultStage)
}

// Switch is a convenience method for string-based enum
func (ib *InstrumentedBuilder) Switch(
	selector func(context.Context) string,
	cases map[string]state.NewStage,
	defaultStage state.NewStage,
) state.NewStage {
	return ib.builder.InstrumentEnum(selector, cases, defaultStage)
}

// Action creates an instrumented action stage
func (ib *InstrumentedBuilder) Action(action func(context.Context)) state.NewStage {
	return ib.builder.InstrumentAction(action)
}

// BuildMachine returns the final XState machine
func (ib *InstrumentedBuilder) BuildMachine() *XStateMachine {
	return ib.builder.BuildMachine()
}

// GetBuilder returns the underlying XState builder
func (ib *InstrumentedBuilder) GetBuilder() *XStateBuilder {
	return ib.builder
}
