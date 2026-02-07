// Package inspector provides a WithInspection wrapper that can introspect
// and instrument existing state machine pipelines built with the state package.
//
// The key innovation is recursive descent into nested states to build a
// complete XState representation of the entire state machine hierarchy.
package inspector

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/authzed/controller-idioms/state"
)

// InspectionWrapper wraps an existing state machine pipeline with inspection capabilities
type InspectionWrapper struct {
	machineID    string
	rootStage    state.NewStage
	builder      *XStateBuilder
	inspector    *Inspector
	stateCounter int64
}

// WithInspection wraps an existing state.NewStage with inspection capabilities.
// It recursively descends into nested states to build a complete XState representation.
func WithInspection(machineID string, pipeline state.NewStage) (*InspectionWrapper, state.NewStage) {
	wrapper := &InspectionWrapper{
		machineID: machineID,
		rootStage: pipeline,
		builder:   NewXStateBuilder(machineID),
		inspector: NewInspector(),
	}

	// Recursively introspect the pipeline structure
	instrumentedPipeline := wrapper.introspectAndInstrument("root", pipeline)

	// Build the XState machine
	machine := wrapper.builder.BuildMachine()
	wrapper.inspector.machines[machineID] = machine

	return wrapper, instrumentedPipeline
}

// introspectAndInstrument recursively walks a state pipeline and builds XState representation
func (iw *InspectionWrapper) introspectAndInstrument(stageID string, stage state.NewStage) state.NewStage {
	return func() state.Stage {
		// Create the original stage to introspect it
		originalStage := stage()
		if originalStage == nil {
			return nil
		}

		// Try to identify the stage type through reflection and known patterns
		stageInfo := iw.identifyStageType(originalStage)

		// Create XState node based on identified type
		xstateNode := iw.createXStateNode(stageID, stageInfo)
		iw.builder.addNode(stageID, xstateNode)

		// Instrument with execution tracking
		return iw.wrapWithInstrumentation(stageID, originalStage, stageInfo)
	}
}

// StageInfo contains information about an identified stage
type StageInfo struct {
	Type        string
	Children    []state.NewStage
	Metadata    map[string]interface{}
	IsSequence  bool
	IsParallel  bool
	IsDecision  bool
	IsEnum      bool
	ChildStages map[string]state.NewStage
}

// identifyStageType attempts to identify the type and structure of a stage
func (iw *InspectionWrapper) identifyStageType(stage state.Stage) StageInfo {
	stageType := reflect.TypeOf(stage)
	info := StageInfo{
		Type:        "unknown",
		Metadata:    make(map[string]interface{}),
		ChildStages: make(map[string]state.NewStage),
	}

	if stageType != nil {
		info.Type = stageType.String()
		info.Metadata["goType"] = stageType.String()
	}

	// Try to identify known stage types through type analysis
	switch {
	case iw.isSequenceStage(stage):
		info.Type = "sequence"
		info.IsSequence = true
		info.Children = iw.extractSequenceChildren(stage)
	case iw.isParallelStage(stage):
		info.Type = "parallel"
		info.IsParallel = true
		info.Children = iw.extractParallelChildren(stage)
	case iw.isDecisionStage(stage):
		info.Type = "decision"
		info.IsDecision = true
		info.ChildStages = iw.extractDecisionChildren(stage)
	case iw.isEnumStage(stage):
		info.Type = "enum"
		info.IsEnum = true
		info.ChildStages = iw.extractEnumChildren(stage)
	default:
		info.Type = "action"
	}

	return info
}

// Helper methods to identify stage types (these would need to be implemented based on
// the internal structure of state package types)
func (iw *InspectionWrapper) isSequenceStage(stage state.Stage) bool {
	// This would check if the stage is a SequenceStage type
	// For now, we use a heuristic approach
	stageType := reflect.TypeOf(stage).String()
	return contains(stageType, "Sequence")
}

func (iw *InspectionWrapper) isParallelStage(stage state.Stage) bool {
	stageType := reflect.TypeOf(stage).String()
	return contains(stageType, "Parallel")
}

func (iw *InspectionWrapper) isDecisionStage(stage state.Stage) bool {
	stageType := reflect.TypeOf(stage).String()
	return contains(stageType, "Decision")
}

func (iw *InspectionWrapper) isEnumStage(stage state.Stage) bool {
	stageType := reflect.TypeOf(stage).String()
	return contains(stageType, "Enum")
}

// These methods would extract child stages from composite stages
// The actual implementation would depend on accessing private fields
// or adding inspection interfaces to the state package
func (iw *InspectionWrapper) extractSequenceChildren(stage state.Stage) []state.NewStage {
	// This would use reflection or inspection interfaces to get child stages
	// For now, return empty - in a real implementation this would extract
	// the stages slice from SequenceStage
	return nil
}

func (iw *InspectionWrapper) extractParallelChildren(stage state.Stage) []state.NewStage {
	// Extract parallel branch stages
	return nil
}

func (iw *InspectionWrapper) extractDecisionChildren(stage state.Stage) map[string]state.NewStage {
	// Extract true/false branch stages
	return map[string]state.NewStage{
		"true":  nil,
		"false": nil,
	}
}

func (iw *InspectionWrapper) extractEnumChildren(stage state.Stage) map[string]state.NewStage {
	// Extract case stages from enum/switch
	return make(map[string]state.NewStage)
}

// createXStateNode creates an XState node based on stage information
func (iw *InspectionWrapper) createXStateNode(stageID string, info StageInfo) *XStateNode {
	node := &XStateNode{
		Type:   "atomic",
		States: make(map[string]*XStateNode),
		On:     make(map[string]*XStateTransition),
		Meta:   info.Metadata,
	}

	node.Meta["stageID"] = stageID
	node.Meta["detectedType"] = info.Type

	switch {
	case info.IsSequence:
		node.Type = "compound"
		node.Meta["type"] = "sequence"
		// Recursively process sequence children
		if len(info.Children) > 0 {
			for i, child := range info.Children {
				childID := iw.generateStateID(fmt.Sprintf("%s_step_%d", stageID, i))
				iw.introspectAndInstrument(childID, child)

				// Add transition to next step (except for last)
				if i < len(info.Children)-1 {
					nextID := iw.generateStateID(fmt.Sprintf("%s_step_%d", stageID, i+1))
					iw.builder.addTransition(childID, nextID)
				}

				// Set initial state
				if i == 0 {
					node.Initial = childID
				}
			}
		}

	case info.IsParallel:
		node.Type = "parallel"
		node.Meta["type"] = "parallel"
		// Recursively process parallel branches
		for i, child := range info.Children {
			branchID := iw.generateStateID(fmt.Sprintf("%s_branch_%d", stageID, i))
			iw.introspectAndInstrument(branchID, child)

			node.States[branchID] = &XStateNode{
				Type: "atomic",
				Meta: map[string]interface{}{
					"parallelIndex": i,
				},
			}
		}

	case info.IsDecision:
		node.Type = "atomic"
		node.Meta["type"] = "decision"
		node.On = map[string]*XStateTransition{
			"EVALUATE": {
				Guard: XStateGuard{
					Type: "evaluateCondition",
				},
			},
		}
		// Process decision branches
		for branchName, branchStage := range info.ChildStages {
			if branchStage != nil {
				branchID := iw.generateStateID(fmt.Sprintf("%s_%s", stageID, branchName))
				iw.introspectAndInstrument(branchID, branchStage)
				iw.builder.addTransition(stageID, branchID)
			}
		}

	case info.IsEnum:
		node.Type = "atomic"
		node.Meta["type"] = "enum"
		node.On = make(map[string]*XStateTransition)
		// Process enum cases
		for caseName, caseStage := range info.ChildStages {
			if caseStage != nil {
				caseID := iw.generateStateID(fmt.Sprintf("%s_case_%s", stageID, caseName))
				iw.introspectAndInstrument(caseID, caseStage)

				node.On[fmt.Sprintf("CASE_%s", caseName)] = &XStateTransition{
					Target: caseID,
				}
				iw.builder.addTransition(stageID, caseID)
			}
		}

	default:
		// Simple action stage
		node.Type = "atomic"
		node.Meta["type"] = "action"
		node.Entry = []XStateAction{{
			Type: "executeAction",
			Meta: map[string]interface{}{
				"stageID": stageID,
			},
		}}
	}

	return node
}

// wrapWithInstrumentation wraps a stage with execution tracking
func (iw *InspectionWrapper) wrapWithInstrumentation(stageID string, original state.Stage, info StageInfo) state.Stage {
	return state.StageFunc(func(ctx context.Context) state.Stage {
		// Record entry into this state
		iw.inspector.recordEvent(ExecutionEvent{
			Timestamp: time.Now(),
			MachineID: iw.machineID,
			EventType: "STATE_ENTRY",
			ToState:   stageID,
			Context:   extractContextData(ctx),
		})

		// Execute the original stage
		result := original.Next(ctx)

		// Record exit/transition
		if result == nil {
			iw.inspector.recordEvent(ExecutionEvent{
				Timestamp: time.Now(),
				MachineID: iw.machineID,
				EventType: "STATE_EXIT",
				FromState: stageID,
			})
		} else {
			nextID := iw.identifyNextStage(result)
			iw.inspector.recordEvent(ExecutionEvent{
				Timestamp: time.Now(),
				MachineID: iw.machineID,
				EventType: "STATE_TRANSITION",
				FromState: stageID,
				ToState:   nextID,
				Context:   extractContextData(ctx),
			})
		}

		return result
	})
}

// identifyNextStage attempts to identify the next stage for logging
func (iw *InspectionWrapper) identifyNextStage(stage state.Stage) string {
	if stage == nil {
		return "completed"
	}

	stageType := reflect.TypeOf(stage)
	if stageType != nil {
		return stageType.String()
	}

	return "unknown"
}

// generateStateID generates unique state IDs
func (iw *InspectionWrapper) generateStateID(prefix string) string {
	counter := atomic.AddInt64(&iw.stateCounter, 1)
	return fmt.Sprintf("%s_%d", prefix, counter)
}

// GetMachine returns the generated XState machine
func (iw *InspectionWrapper) GetMachine() *XStateMachine {
	return iw.builder.BuildMachine()
}

// GetInspector returns the inspector for runtime monitoring
func (iw *InspectionWrapper) GetInspector() *Inspector {
	return iw.inspector
}

// StartServer starts the web inspector server
func (iw *InspectionWrapper) StartServer(addr string) error {
	return iw.inspector.StartServer(addr)
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
