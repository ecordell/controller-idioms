// Package inspector provides tools for translating state.Stage hierarchical
// state machines into XState-compatible machine definitions and serving
// a web-based inspector for live debugging of controller state machines.
//
// This package bridges the gap between Go's continuation-passing style
// state machines and XState's JSON-based state machine definitions,
// enabling visual inspection and debugging of complex controller logic.
package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/authzed/controller-idioms/state"
)

// XStateNode represents a single state in an XState machine definition
type XStateNode struct {
	Type    string                       `json:"type,omitempty"`
	Initial string                       `json:"initial,omitempty"`
	States  map[string]*XStateNode       `json:"states,omitempty"`
	On      map[string]*XStateTransition `json:"on,omitempty"`
	Entry   []XStateAction               `json:"entry,omitempty"`
	Exit    []XStateAction               `json:"exit,omitempty"`
	Always  []*XStateTransition          `json:"always,omitempty"`
	After   map[string]*XStateTransition `json:"after,omitempty"`
	Tags    []string                     `json:"tags,omitempty"`
	Meta    map[string]interface{}       `json:"meta,omitempty"`
}

// XStateMachine represents a complete XState machine definition
type XStateMachine struct {
	ID      string                 `json:"id"`
	Initial string                 `json:"initial"`
	States  map[string]*XStateNode `json:"states"`
	Context map[string]interface{} `json:"context,omitempty"`
	Version string                 `json:"version,omitempty"`
}

// XStateTransition represents a transition between states
type XStateTransition struct {
	Target  string         `json:"target,omitempty"`
	Actions []XStateAction `json:"actions,omitempty"`
	Cond    string         `json:"cond,omitempty"`
	Guard   XStateGuard    `json:"guard,omitempty"`
}

// XStateAction represents an action that can be executed
type XStateAction struct {
	Type string                 `json:"type"`
	Exec string                 `json:"exec,omitempty"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// XStateGuard represents a guard condition
type XStateGuard struct {
	Type string                 `json:"type"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// Inspector provides functionality for translating and inspecting state machines
type Inspector struct {
	mu               sync.RWMutex
	machines         map[string]*XStateMachine
	currentStates    map[string]string
	executionHistory []ExecutionEvent
	stateAnalyzer    *StateAnalyzer
}

// ExecutionEvent represents a single event in the state machine execution
type ExecutionEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	MachineID string                 `json:"machineId"`
	EventType string                 `json:"eventType"`
	FromState string                 `json:"fromState,omitempty"`
	ToState   string                 `json:"toState,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// StateAnalyzer analyzes state.Stage structures to extract state machine information
type StateAnalyzer struct {
	stateCounter int
	stateMap     map[reflect.Type]string
	mu           sync.Mutex
}

// NewInspector creates a new Inspector instance
func NewInspector() *Inspector {
	return &Inspector{
		machines:         make(map[string]*XStateMachine),
		currentStates:    make(map[string]string),
		executionHistory: make([]ExecutionEvent, 0),
		stateAnalyzer:    NewStateAnalyzer(),
	}
}

// NewStateAnalyzer creates a new StateAnalyzer
func NewStateAnalyzer() *StateAnalyzer {
	return &StateAnalyzer{
		stateCounter: 0,
		stateMap:     make(map[reflect.Type]string),
	}
}

// AnalyzeStage analyzes a state.NewStage and generates an XState machine definition
func (i *Inspector) AnalyzeStage(machineID string, rootStage state.NewStage) (*XStateMachine, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	machine := &XStateMachine{
		ID:      machineID,
		Initial: "root",
		States:  make(map[string]*XStateNode),
		Context: make(map[string]interface{}),
		Version: "5",
	}

	// Analyze the root stage
	rootNode, err := i.stateAnalyzer.analyzeNewStage(rootStage, "root")
	if err != nil {
		return nil, fmt.Errorf("failed to analyze root stage: %w", err)
	}

	machine.States["root"] = rootNode
	i.machines[machineID] = machine

	return machine, nil
}

// analyzeNewStage recursively analyzes a NewStage and builds XState nodes
func (sa *StateAnalyzer) analyzeNewStage(newStage state.NewStage, stateID string) (*XStateNode, error) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	node := &XStateNode{
		Type:   "atomic",
		States: make(map[string]*XStateNode),
		On:     make(map[string]*XStateTransition),
		Meta:   make(map[string]interface{}),
	}

	// Try to determine the stage type through reflection and type analysis
	stageType := reflect.TypeOf(newStage)
	node.Meta["goType"] = stageType.String()
	node.Meta["stateID"] = stateID

	// For now, we'll use a simplified analysis
	// In a full implementation, this would need more sophisticated introspection
	node.Entry = []XStateAction{{
		Type: "executeStage",
		Meta: map[string]interface{}{
			"stageID": stateID,
		},
	}}

	// Add automatic transition for simple stages
	node.Always = []*XStateTransition{{
		Target: "completed",
	}}

	// Add a completion state
	node.Type = "compound"
	node.Initial = "executing"
	node.States["executing"] = &XStateNode{
		Type: "atomic",
		Entry: []XStateAction{{
			Type: "executeStage",
		}},
		Always: []*XStateTransition{{
			Target: "completed",
		}},
	}
	node.States["completed"] = &XStateNode{
		Type: "final",
	}

	return node, nil
}

// InstrumentStage wraps a stage with instrumentation for live inspection
func (i *Inspector) InstrumentStage(machineID, stageID string, stage state.NewStage) state.NewStage {
	return func() state.Stage {
		return state.StageFunc(func(ctx context.Context) state.Stage {
			// Record entry into this state
			i.recordEvent(ExecutionEvent{
				Timestamp: time.Now(),
				MachineID: machineID,
				EventType: "STATE_ENTRY",
				ToState:   stageID,
				Context:   extractContextData(ctx),
			})

			// Execute the original stage
			originalStage := stage()
			if originalStage == nil {
				// Record completion
				i.recordEvent(ExecutionEvent{
					Timestamp: time.Now(),
					MachineID: machineID,
					EventType: "STATE_EXIT",
					FromState: stageID,
				})
				return nil
			}

			// Execute and capture result
			result := originalStage.Next(ctx)

			// Record transition
			nextStateID := "unknown"
			if result != nil {
				nextStateID = i.identifyStage(result)
			}

			i.recordEvent(ExecutionEvent{
				Timestamp: time.Now(),
				MachineID: machineID,
				EventType: "STATE_TRANSITION",
				FromState: stageID,
				ToState:   nextStateID,
				Context:   extractContextData(ctx),
			})

			return result
		})
	}
}

// identifyStage attempts to identify a stage for debugging purposes
func (i *Inspector) identifyStage(stage state.Stage) string {
	stageType := reflect.TypeOf(stage)
	if stageType != nil {
		return stageType.String()
	}
	return "unknown"
}

// recordEvent adds an execution event to the history
func (i *Inspector) recordEvent(event ExecutionEvent) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.executionHistory = append(i.executionHistory, event)

	// Update current state
	if event.EventType == "STATE_TRANSITION" && event.ToState != "" {
		i.currentStates[event.MachineID] = event.ToState
	}

	// Keep history bounded (last 1000 events)
	if len(i.executionHistory) > 1000 {
		i.executionHistory = i.executionHistory[len(i.executionHistory)-1000:]
	}
}

// extractContextData extracts relevant data from a Go context for debugging
func extractContextData(ctx context.Context) map[string]interface{} {
	data := make(map[string]interface{})

	// Extract some standard context values
	if deadline, ok := ctx.Deadline(); ok {
		data["deadline"] = deadline.Format(time.RFC3339)
	}

	if ctx.Err() != nil {
		data["error"] = ctx.Err().Error()
	}

	// Note: In a real implementation, you'd want to extract specific
	// context values that are relevant to your controllers
	return data
}

// GetMachine returns the XState machine definition for a given machine ID
func (i *Inspector) GetMachine(machineID string) (*XStateMachine, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	machine, exists := i.machines[machineID]
	return machine, exists
}

// GetCurrentState returns the current state for a given machine ID
func (i *Inspector) GetCurrentState(machineID string) (string, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()

	state, exists := i.currentStates[machineID]
	return state, exists
}

// GetExecutionHistory returns the recent execution history
func (i *Inspector) GetExecutionHistory(machineID string, limit int) []ExecutionEvent {
	i.mu.RLock()
	defer i.mu.RUnlock()

	var filtered []ExecutionEvent
	for j := len(i.executionHistory) - 1; j >= 0 && len(filtered) < limit; j-- {
		event := i.executionHistory[j]
		if event.MachineID == machineID {
			filtered = append([]ExecutionEvent{event}, filtered...)
		}
	}

	return filtered
}

// ServeHTTP serves the inspector web interface
func (i *Inspector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/machines":
		i.handleMachines(w, r)
	case "/api/state":
		i.handleCurrentState(w, r)
	case "/api/history":
		i.handleHistory(w, r)
	case "/":
		i.handleIndex(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleMachines returns all machine definitions
func (i *Inspector) handleMachines(w http.ResponseWriter, r *http.Request) {
	i.mu.RLock()
	machines := make(map[string]*XStateMachine)
	for id, machine := range i.machines {
		machines[id] = machine
	}
	i.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(machines)
}

// handleCurrentState returns the current state of all machines
func (i *Inspector) handleCurrentState(w http.ResponseWriter, r *http.Request) {
	i.mu.RLock()
	states := make(map[string]string)
	for id, state := range i.currentStates {
		states[id] = state
	}
	i.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(states)
}

// handleHistory returns execution history
func (i *Inspector) handleHistory(w http.ResponseWriter, r *http.Request) {
	machineID := r.URL.Query().Get("machine")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := fmt.Sscanf(l, "%d", &limit); err == nil && parsed == 1 {
			// limit parsed successfully
		}
	}

	history := i.GetExecutionHistory(machineID, limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// handleIndex serves the main inspector interface
func (i *Inspector) handleIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>State Machine Inspector</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .machine { border: 1px solid #ccc; padding: 15px; margin: 10px 0; }
        .state { background: #f0f0f0; padding: 5px; margin: 5px 0; }
        .current { background: #90EE90; }
        .history { max-height: 300px; overflow-y: scroll; }
        .event { font-size: 12px; padding: 2px; border-bottom: 1px solid #eee; }
    </style>
</head>
<body>
    <h1>State Machine Inspector</h1>
    <div id="machines"></div>
    <h2>Execution History</h2>
    <div id="history" class="history"></div>

    <script>
        async function fetchData() {
            try {
                const [machines, states, history] = await Promise.all([
                    fetch('/api/machines').then(r => r.json()),
                    fetch('/api/state').then(r => r.json()),
                    fetch('/api/history').then(r => r.json())
                ]);

                updateUI(machines, states, history);
            } catch (error) {
                console.error('Error fetching data:', error);
            }
        }

        function updateUI(machines, states, history) {
            const machinesDiv = document.getElementById('machines');
            machinesDiv.innerHTML = '';

            for (const [id, machine] of Object.entries(machines)) {
                const div = document.createElement('div');
                div.className = 'machine';
                div.innerHTML = '<h3>' + machine.id + '</h3>' +
                               '<p>Current State: <span class="current">' + (states[id] || 'unknown') + '</span></p>' +
                               '<pre>' + JSON.stringify(machine, null, 2) + '</pre>';
                machinesDiv.appendChild(div);
            }

            const historyDiv = document.getElementById('history');
            historyDiv.innerHTML = '';

            history.forEach(event => {
                const div = document.createElement('div');
                div.className = 'event';
                div.textContent = event.timestamp + ' - ' + event.machineId + ': ' +
                                 event.eventType + ' ' + (event.fromState || '') +
                                 (event.toState ? ' -> ' + event.toState : '');
                historyDiv.appendChild(div);
            });
        }

        // Auto-refresh every 2 seconds
        setInterval(fetchData, 2000);
        fetchData();
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// StartServer starts the inspector HTTP server
func (i *Inspector) StartServer(addr string) error {
	http.Handle("/", i)
	return http.ListenAndServe(addr, nil)
}
