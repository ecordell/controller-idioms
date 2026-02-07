// Package inspector provides real-time visualization of Go state machines
// using the Stately Inspector (@statelyai/inspect) for live debugging.
//
// This integrates our Go state machine introspection with the browser-based
// Stately Inspector to provide real-time visualization of controller state machines.
package inspector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/authzed/controller-idioms/state"
)

// RealTimeInspector provides live visualization of state machines using Stately Inspector
type RealTimeInspector struct {
	mu              sync.RWMutex
	machines        map[string]*XStateMachine
	connections     map[string]*websocket.Conn
	eventBuffer     []InspectorEvent
	maxBufferSize   int
	upgrader        websocket.Upgrader
	statelyAssets   map[string][]byte
	currentStates   map[string]string
	executionEvents map[string][]ExecutionEvent
}

// InspectorEvent represents an event sent to the Stately Inspector
type InspectorEvent struct {
	Type      string                 `json:"type"`
	ActorID   string                 `json:"actorId,omitempty"`
	SessionID string                 `json:"sessionId,omitempty"`
	Event     interface{}            `json:"event,omitempty"`
	Snapshot  interface{}            `json:"snapshot,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NewRealTimeInspector creates a new real-time inspector
func NewRealTimeInspector() *RealTimeInspector {
	return &RealTimeInspector{
		machines:        make(map[string]*XStateMachine),
		connections:     make(map[string]*websocket.Conn),
		eventBuffer:     make([]InspectorEvent, 0),
		maxBufferSize:   1000,
		currentStates:   make(map[string]string),
		executionEvents: make(map[string][]ExecutionEvent),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
		statelyAssets: loadStatelyAssets(),
	}
}

// WithRealTimeInspection wraps a state pipeline with live Stately Inspector integration
func WithRealTimeInspection(machineID string, pipeline state.NewStage) (*RealTimeInspector, state.NewStage) {
	inspector := NewRealTimeInspector()

	// First, analyze the pipeline structure
	wrapper := &InspectionWrapper{
		machineID: machineID,
		rootStage: pipeline,
		builder:   NewXStateBuilder(machineID),
		inspector: NewInspector(),
	}

	// Introspect the pipeline to build XState representation
	instrumentedPipeline := wrapper.introspectAndInstrument("root", pipeline)
	machine := wrapper.builder.BuildMachine()

	// Register with real-time inspector
	inspector.RegisterMachine(machineID, machine)

	// Wrap with real-time event streaming
	realtimeInstrumentedPipeline := inspector.WrapWithRealTimeTracking(machineID, instrumentedPipeline)

	return inspector, realtimeInstrumentedPipeline
}

// RegisterMachine registers a state machine with the inspector
func (rt *RealTimeInspector) RegisterMachine(machineID string, machine *XStateMachine) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.machines[machineID] = machine
	rt.currentStates[machineID] = machine.Initial
	rt.executionEvents[machineID] = make([]ExecutionEvent, 0)

	// Send machine registration event
	event := InspectorEvent{
		Type:      "xstate.actor",
		ActorID:   machineID,
		SessionID: "go-controller-session",
		Snapshot: map[string]interface{}{
			"status": "active",
			"value":  machine.Initial,
			"context": map[string]interface{}{
				"machineId": machineID,
				"goType":    "state.NewStage",
			},
		},
		Context:   machine.Context,
		Timestamp: time.Now(),
	}

	rt.broadcastEvent(event)
}

// WrapWithRealTimeTracking wraps a pipeline with real-time event streaming
func (rt *RealTimeInspector) WrapWithRealTimeTracking(machineID string, pipeline state.NewStage) state.NewStage {
	return func() state.Stage {
		return state.StageFunc(func(ctx context.Context) state.Stage {
			// Execute the original pipeline
			originalStage := pipeline()
			if originalStage == nil {
				return nil
			}

			// Send state entry event
			rt.sendStateEvent(machineID, "entry", extractStateFromStage(originalStage), ctx)

			// Execute and capture result
			result := originalStage.Next(ctx)

			// Send transition event
			if result == nil {
				rt.sendStateEvent(machineID, "exit", "completed", ctx)
			} else {
				nextState := extractStateFromStage(result)
				rt.sendTransitionEvent(machineID, extractStateFromStage(originalStage), nextState, ctx)
			}

			return result
		})
	}
}

// sendStateEvent sends a state entry/exit event to connected clients
func (rt *RealTimeInspector) sendStateEvent(machineID, eventType, state string, ctx context.Context) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.currentStates[machineID] = state

	event := InspectorEvent{
		Type:      "xstate.event",
		ActorID:   machineID,
		SessionID: "go-controller-session",
		Event: map[string]interface{}{
			"type": eventType,
		},
		Snapshot: map[string]interface{}{
			"status":  "active",
			"value":   state,
			"context": extractContextForInspector(ctx),
		},
		Timestamp: time.Now(),
	}

	rt.broadcastEvent(event)
}

// sendTransitionEvent sends a state transition event
func (rt *RealTimeInspector) sendTransitionEvent(machineID, fromState, toState string, ctx context.Context) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.currentStates[machineID] = toState

	event := InspectorEvent{
		Type:      "xstate.event",
		ActorID:   machineID,
		SessionID: "go-controller-session",
		Event: map[string]interface{}{
			"type": "TRANSITION",
			"from": fromState,
			"to":   toState,
		},
		Snapshot: map[string]interface{}{
			"status":  "active",
			"value":   toState,
			"context": extractContextForInspector(ctx),
		},
		Timestamp: time.Now(),
	}

	rt.broadcastEvent(event)
}

// broadcastEvent sends an event to all connected WebSocket clients
func (rt *RealTimeInspector) broadcastEvent(event InspectorEvent) {
	// Add to buffer
	rt.eventBuffer = append(rt.eventBuffer, event)
	if len(rt.eventBuffer) > rt.maxBufferSize {
		rt.eventBuffer = rt.eventBuffer[len(rt.eventBuffer)-rt.maxBufferSize:]
	}

	// Broadcast to all connected clients
	for clientID, conn := range rt.connections {
		if conn != nil {
			if err := conn.WriteJSON(event); err != nil {
				log.Printf("Error sending event to client %s: %v", clientID, err)
				conn.Close()
				delete(rt.connections, clientID)
			}
		}
	}
}

// ServeHTTP implements the HTTP handler for the inspector web interface
func (rt *RealTimeInspector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
		rt.serveInspectorUI(w, r)
	case "/ws":
		rt.handleWebSocket(w, r)
	case "/api/machines":
		rt.handleAPIMachines(w, r)
	case "/api/events":
		rt.handleAPIEvents(w, r)
	default:
		// Serve Stately Inspector assets
		rt.serveAsset(w, r)
	}
}

// serveInspectorUI serves the main inspector interface
func (rt *RealTimeInspector) serveInspectorUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Real-Time State Machine Inspector</title>
    <script>
        // Create custom state machine visualizer
        function loadStatelyInspect() {
            console.log('🚀 Setting up custom state machine visualizer...');

            if (inspector) {
                console.log('ℹ️ Inspector already initialized, skipping');
                return;
            }

            createStateVisualizer();

            // Create inspector that updates the visualization
            inspector = {
                currentState: 'idle',
                states: new Set(),
                transitions: [],

                actor: function(actorId, snapshot) {
                    console.log('Actor registered:', actorId, snapshot);
                    this.updateVisualization('actor', { actorId, snapshot });
                },

                event: function(actorId, event, meta) {
                    console.log('Event received:', actorId, event, meta);
                    if (event && event.type) {
                        this.addTransition(this.currentState, event.type);
                        this.updateVisualization('event', { actorId, event, meta });
                    }
                },

                snapshot: function(actorId, snapshot) {
                    console.log('State changed:', actorId, snapshot);
                    if (snapshot && snapshot.value && snapshot.value !== this.currentState) {
                        this.currentState = snapshot.value;
                        this.states.add(snapshot.value);
                        this.updateVisualization('state', { actorId, snapshot });
                    }
                },

                addTransition: function(from, event) {
                    const transition = { from, event, timestamp: Date.now() };
                    this.transitions.push(transition);
                    if (this.transitions.length > 50) {
                        this.transitions = this.transitions.slice(-50);
                    }
                },

                updateVisualization: function(type, data) {
                    this.renderStateMachine();
                    this.renderEventLog(type, data);
                },

                renderStateMachine: function() {
                    const container = document.getElementById('state-machine');
                    const states = Array.from(this.states);

                    let html = '<div class="state-graph">';

                    states.forEach((state, index) => {
                        const isActive = state === this.currentState;
                        const stateClass = isActive ? 'state active' : 'state';
                        const delay = index * 100;

                        html +=
                            '<div class="' + stateClass + '" style="animation-delay: ' + delay + 'ms">' +
                                '<div class="state-name">' + state + '</div>' +
                                (isActive ? '<div class="state-indicator"></div>' : '') +
                            '</div>';

                        if (index < states.length - 1) {
                            html += '<div class="transition-arrow">→</div>';
                        }
                    });

                    html += '</div>';
                    container.innerHTML = html;
                },

                renderEventLog: function(type, data) {
                    const logContainer = document.getElementById('event-log');
                    const timestamp = new Date().toLocaleTimeString();

                    let eventHtml = '';
                    switch(type) {
                        case 'actor':
                            eventHtml = '<div class="log-entry actor"><span class="timestamp">' + timestamp + '</span> <span class="event-type">ACTOR</span> ' + data.actorId + ' registered</div>';
                            break;
                        case 'event':
                            eventHtml = '<div class="log-entry event"><span class="timestamp">' + timestamp + '</span> <span class="event-type">EVENT</span> ' + data.event.type + '</div>';
                            break;
                        case 'state':
                            eventHtml = '<div class="log-entry state"><span class="timestamp">' + timestamp + '</span> <span class="event-type">STATE</span> ' + data.snapshot.value + '</div>';
                            break;
                    }

                    logContainer.insertAdjacentHTML('afterbegin', eventHtml);

                    // Keep only last 20 entries
                    const entries = logContainer.querySelectorAll('.log-entry');
                    if (entries.length > 20) {
                        entries[entries.length - 1].remove();
                    }
                }
            };

            console.log('✅ Custom visualizer ready, inspector object:', inspector);
        }

        function createStateVisualizer() {
            const inspectorContainer = document.getElementById('inspector');
            inspectorContainer.innerHTML =
                '<div class="visualizer-container">' +
                    '<div class="visualizer-header">' +
                        '<h3>State Machine Visualizer</h3>' +
                        '<div class="status-indicators">' +
                            '<div class="indicator active">Live</div>' +
                        '</div>' +
                    '</div>' +

                    '<div class="visualizer-content">' +
                        '<div class="state-section">' +
                            '<h4>Current State Flow</h4>' +
                            '<div id="state-machine" class="state-machine-container">' +
                                '<div class="empty-state">Waiting for state machine events...</div>' +
                            '</div>' +
                        '</div>' +

                        '<div class="log-section">' +
                            '<h4>Event Log</h4>' +
                            '<div id="event-log" class="event-log">' +
                                '<div class="log-entry info">Visualizer initialized - waiting for events</div>' +
                            '</div>' +
                        '</div>' +
                    '</div>' +
                '</div>';
        }

        function initFallbackInspector() {
            console.warn('Initializing fallback inspector');
            inspector = {
                actor: function(id, data) {
                    console.log('Fallback Actor:', id, data);
                },
                event: function(id, event, meta) {
                    console.log('Fallback Event:', id, event, meta);
                },
                snapshot: function(id, snapshot) {
                    console.log('Fallback Snapshot:', id, snapshot);
                }
            };

            document.getElementById('inspector').innerHTML =
                '<div style="padding: 20px; text-align: center; color: #666; height: 100%;">' +
                '<h3>🔍 Inspector Console Mode</h3>' +
                '<p>Stately Inspector UI not available - check browser console for events</p>' +
                '<div style="background: #f5f5f5; padding: 15px; border-radius: 8px; margin-top: 20px;">' +
                '<strong>Open Developer Tools (F12) to see:</strong><br>' +
                '&bull; State machine events<br>' +
                '&bull; Actor registrations<br>' +
                '&bull; Transition logs' +
                '</div></div>';
        }

        // Initialize when page loads
        document.addEventListener('DOMContentLoaded', function() {
            console.log('📄 DOMContentLoaded - initializing inspector');
            loadStatelyInspect();
        });

        // Fallback for older browsers
        window.addEventListener('load', function() {
            console.log('🌐 Window loaded');
            if (!inspector) {
                console.log('⚠️ Inspector not found after DOMContentLoaded, trying again');
                loadStatelyInspect();
            } else {
                console.log('✅ Inspector already initialized');
            }
        });
    </script>
    <style>
        body {
            margin: 0;
            padding: 20px;
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
            background: #f5f5f5;
        }

        /* Custom Visualizer Styles */
        .visualizer-container {
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            overflow: hidden;
        }

        .visualizer-header {
            background: linear-gradient(90deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .visualizer-header h3 {
            margin: 0;
            font-size: 1.2em;
        }

        .status-indicators {
            display: flex;
            gap: 10px;
        }

        .indicator {
            padding: 4px 12px;
            border-radius: 12px;
            background: rgba(255,255,255,0.2);
            font-size: 0.8em;
        }

        .indicator.active {
            background: #10b981;
            animation: pulse 2s infinite;
        }

        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.7; }
        }

        .visualizer-content {
            padding: 0;
        }

        .state-section, .log-section {
            padding: 20px;
        }

        .state-section {
            border-bottom: 1px solid #e1e5e9;
        }

        .state-section h4, .log-section h4 {
            margin: 0 0 15px 0;
            color: #374151;
            font-size: 1em;
            font-weight: 600;
        }

        .state-machine-container {
            min-height: 100px;
            background: #f8fafc;
            border-radius: 8px;
            padding: 20px;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .state-graph {
            display: flex;
            align-items: center;
            gap: 15px;
            flex-wrap: wrap;
        }

        .state {
            position: relative;
            background: #e2e8f0;
            border: 2px solid #cbd5e0;
            border-radius: 8px;
            padding: 12px 18px;
            transition: all 0.3s ease;
            animation: slideIn 0.5s ease-out;
        }

        .state.active {
            background: #10b981;
            border-color: #059669;
            color: white;
            transform: scale(1.05);
            box-shadow: 0 4px 12px rgba(16, 185, 129, 0.3);
        }

        .state-name {
            font-weight: 600;
            font-size: 0.9em;
        }

        .state-indicator {
            position: absolute;
            top: -3px;
            right: -3px;
            width: 8px;
            height: 8px;
            background: #fbbf24;
            border-radius: 50%;
            animation: bounce 1s infinite;
        }

        .transition-arrow {
            font-size: 1.5em;
            color: #6b7280;
            font-weight: bold;
        }

        .empty-state {
            color: #9ca3af;
            font-style: italic;
        }

        @keyframes slideIn {
            from {
                opacity: 0;
                transform: translateY(-10px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        @keyframes bounce {
            0%, 100% { transform: translateY(0); }
            50% { transform: translateY(-3px); }
        }

        .event-log {
            background: #1f2937;
            color: #e5e7eb;
            border-radius: 6px;
            padding: 15px;
            max-height: 300px;
            overflow-y: auto;
            font-family: 'Monaco', 'Consolas', monospace;
            font-size: 0.85em;
        }

        .log-entry {
            padding: 6px 0;
            border-bottom: 1px solid #374151;
            opacity: 0;
            animation: fadeIn 0.3s ease-out forwards;
        }

        .log-entry:last-child {
            border-bottom: none;
        }

        .log-entry.actor { color: #60a5fa; }
        .log-entry.event { color: #fbbf24; }
        .log-entry.state { color: #10b981; }
        .log-entry.info { color: #9ca3af; }

        .timestamp {
            color: #6b7280;
            margin-right: 8px;
        }

        .event-type {
            background: #374151;
            padding: 2px 6px;
            border-radius: 3px;
            font-size: 0.8em;
            margin-right: 8px;
        }

        @keyframes fadeIn {
            from { opacity: 0; transform: translateX(-10px); }
            to { opacity: 1; transform: translateX(0); }
        }
        .header {
            background: white;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .status {
            display: flex;
            gap: 20px;
            margin: 20px 0;
        }
        .status-item {
            background: white;
            padding: 15px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            flex: 1;
        }
        .inspector-container {
            background: white;
            border-radius: 8px;
            padding: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            min-height: 600px;
        }
        .connected { color: #10b981; }
        .disconnected { color: #ef4444; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔍 Real-Time State Machine Inspector</h1>
        <p>Live visualization of Go controller state machines using Stately Inspector</p>
    </div>

    <div class="status">
        <div class="status-item">
            <h3>Connection Status</h3>
            <div id="status" class="disconnected">Connecting...</div>
        </div>
        <div class="status-item">
            <h3>Active Machines</h3>
            <div id="machine-count">0</div>
        </div>
        <div class="status-item">
            <h3>Events Received</h3>
            <div id="event-count">0</div>
        </div>
    </div>

    <div class="inspector-container">
        <div id="inspector"></div>
    </div>

    <script>
        let inspector;

        function initInspector() {
            // Inspector is initialized in loadStatelyInspect
            console.log('Inspector initialization handled by loadStatelyInspect');
            if (!inspector) {
                loadStatelyInspect();
            }
        }

        // WebSocket connection
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(protocol + '//' + window.location.host + '/ws');

        let eventCount = 0;
        let machines = new Set();

        ws.onopen = function() {
            document.getElementById('status').textContent = 'Connected';
            document.getElementById('status').className = 'connected';
        };

        ws.onclose = function() {
            document.getElementById('status').textContent = 'Disconnected';
            document.getElementById('status').className = 'disconnected';
        };

        ws.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                console.log('Received WebSocket event:', data);

                eventCount++;
                document.getElementById('event-count').textContent = eventCount;

                // Track machines by actorId
                if (data.actorId) {
                    machines.add(data.actorId);
                    document.getElementById('machine-count').textContent = machines.size;
                }

                // Initialize inspector if not already done
                if (!inspector) {
                    console.log('⚠️ Inspector not initialized, initializing now...');
                    loadStatelyInspect();
                    // Give it a moment to initialize
                    setTimeout(() => {
                        if (!inspector) {
                            console.error('❌ Inspector still not initialized after retry');
                        }
                    }, 100);
                }

                // Ensure inspector is available and has required methods
                if (!inspector) {
                    console.warn('Inspector still not initialized, logging to console only');
                    console.log('Event data:', data);
                    return;
                }

                // Forward to Stately Inspector with better error handling
                try {
                    if (data.type === 'xstate.actor' && data.actorId && data.snapshot) {
                        console.log('Sending actor to inspector:', data.actorId);
                        if (typeof inspector.actor === 'function') {
                            inspector.actor(data.actorId, data.snapshot);
                        } else {
                            console.warn('Inspector.actor is not a function');
                        }
                    } else if (data.type === 'xstate.event' && data.actorId && data.event) {
                        console.log('Sending event to inspector:', data.actorId, data.event);

                        if (typeof inspector.event === 'function') {
                            inspector.event(data.actorId, data.event, { source: 'go-controller' });
                        } else {
                            console.warn('Inspector.event is not a function');
                        }

                        if (data.snapshot && typeof inspector.snapshot === 'function') {
                            inspector.snapshot(data.actorId, data.snapshot);
                        }
                    }
                } catch (inspectorError) {
                    console.error('Error calling inspector methods:', inspectorError);
                }
            } catch (e) {
                console.error('Error parsing WebSocket message:', e);
                console.error('Raw message:', event.data);
            }
        };

        // Load initial state
        fetch('/api/machines')
            .then(response => response.json())
            .then(machinesData => {
                if (inspector && machinesData) {
                    Object.entries(machinesData).forEach(([id, machine]) => {
                        inspector.actor(id, {
                            status: 'active',
                            value: machine.initial || 'unknown',
                            context: machine.context || {}
                        });
                    });
                }
            })
            .catch(error => {
                console.error('Error loading initial machines:', error);
            });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleWebSocket handles WebSocket connections for real-time events
func (rt *RealTimeInspector) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := rt.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())

	rt.mu.Lock()
	rt.connections[clientID] = conn
	rt.mu.Unlock()

	// Send buffered events to new client
	for _, event := range rt.eventBuffer {
		if err := conn.WriteJSON(event); err != nil {
			log.Printf("Error sending buffered event: %v", err)
			break
		}
	}

	// Handle client disconnect
	defer func() {
		rt.mu.Lock()
		delete(rt.connections, clientID)
		rt.mu.Unlock()
		conn.Close()
	}()

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
	}
}

// handleAPIMachines returns registered machines
func (rt *RealTimeInspector) handleAPIMachines(w http.ResponseWriter, r *http.Request) {
	rt.mu.RLock()
	machines := make(map[string]*XStateMachine)
	for id, machine := range rt.machines {
		machines[id] = machine
	}
	rt.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(machines)
}

// handleAPIEvents returns recent events
func (rt *RealTimeInspector) handleAPIEvents(w http.ResponseWriter, r *http.Request) {
	rt.mu.RLock()
	events := make([]InspectorEvent, len(rt.eventBuffer))
	copy(events, rt.eventBuffer)
	rt.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

// serveAsset serves static assets (placeholder for Stately Inspector assets)
func (rt *RealTimeInspector) serveAsset(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// StartServer starts the real-time inspector server
func (rt *RealTimeInspector) StartServer(addr string) error {
	log.Printf("Starting Real-Time Inspector server on %s", addr)
	log.Printf("Visit http://localhost%s for live state machine visualization", addr)
	return http.ListenAndServe(addr, rt)
}

// Helper functions

// extractStateFromStage attempts to extract a meaningful state name from a stage
func extractStateFromStage(stage state.Stage) string {
	if stage == nil {
		return "completed"
	}
	// Use type information as state identifier
	return fmt.Sprintf("%T", stage)
}

// extractContextForInspector extracts relevant context data for the inspector
func extractContextForInspector(ctx context.Context) map[string]interface{} {
	data := make(map[string]interface{})

	// Add timestamp
	data["timestamp"] = time.Now().Format(time.RFC3339)

	// Add error information if present
	if err := ctx.Err(); err != nil {
		data["error"] = err.Error()
	}

	// Add deadline information if present
	if deadline, ok := ctx.Deadline(); ok {
		data["deadline"] = deadline.Format(time.RFC3339)
	}

	return data
}

// loadStatelyAssets loads Stately Inspector assets (placeholder)
func loadStatelyAssets() map[string][]byte {
	// In a real implementation, you might embed the Stately Inspector assets
	// or serve them from a CDN
	return make(map[string][]byte)
}
