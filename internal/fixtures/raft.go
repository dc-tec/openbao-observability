package fixtures

import (
	"encoding/json"
	"fmt"
)

type raftListPeers struct {
	Config struct {
		Servers []raftServer `json:"servers"`
	} `json:"config"`
	Data struct {
		Config struct {
			Servers []raftServer `json:"servers"`
		} `json:"config"`
	} `json:"data"`
}

type raftServer struct {
	NodeID string `json:"node_id"`
	Leader bool   `json:"leader"`
	Voter  bool   `json:"voter"`
}

func parseRaftServers(content []byte) ([]raftServer, error) {
	var peers raftListPeers
	if err := json.Unmarshal(content, &peers); err != nil {
		return nil, fmt.Errorf("parse Raft peers JSON: %w", err)
	}
	if len(peers.Config.Servers) > 0 {
		return peers.Config.Servers, nil
	}
	return peers.Data.Config.Servers, nil
}

func countVoters(servers []raftServer) int {
	var count int
	for _, server := range servers {
		if server.Voter {
			count++
		}
	}
	return count
}

type autopilotState struct {
	Healthy          bool                       `json:"Healthy"`
	FailureTolerance int                        `json:"FailureTolerance"`
	Servers          map[string]autopilotServer `json:"Servers"`
}

type autopilotServer struct {
	Healthy  bool   `json:"Healthy"`
	NodeType string `json:"NodeType"`
	Status   string `json:"Status"`
}

func parseAutopilotState(content []byte) (autopilotState, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(content, &raw); err != nil {
		return autopilotState{}, fmt.Errorf("parse Autopilot JSON: %w", err)
	}

	var state autopilotState
	if data, ok := raw["data"]; ok {
		if err := json.Unmarshal(data, &state); err != nil {
			return autopilotState{}, fmt.Errorf("parse wrapped Autopilot data: %w", err)
		}
		return state, nil
	}
	if err := json.Unmarshal(content, &state); err != nil {
		return autopilotState{}, fmt.Errorf("parse Autopilot state: %w", err)
	}
	return state, nil
}
