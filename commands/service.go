package commands

import (
	"context"

	"github.com/bluemedora/bplogagent/agent"
	"github.com/kardianos/service"
	"go.uber.org/zap"
)

// AgentService is a service that runs the log agent.
type AgentService struct {
	cancel context.CancelFunc
	agent  *agent.LogAgent
}

// Start will start the log agent.
func (a *AgentService) Start(s service.Service) error {
	a.agent.Info("Starting log agent")
	if err := a.agent.Start(); err != nil {
		a.agent.Errorw("Failed to start log agent", zap.Any("error", err))
		a.cancel()
	}
	return nil
}

// Stop will stop the log agent.
func (a *AgentService) Stop(s service.Service) error {
	a.agent.Info("Stopping log agent")
	a.agent.Stop()
	a.cancel()
	return nil
}

// newAgentService creates a new agent service with the provided agent.
func newAgentService(agent *agent.LogAgent, cancel context.CancelFunc) (service.Service, error) {
	agentService := &AgentService{cancel, agent}
	config := &service.Config{
		Name:        "bplogagent",
		DisplayName: "bplogagent",
		Description: "Monitors and processes log entries",
	}

	service, err := service.New(agentService, config)
	if err != nil {
		return nil, err
	}

	return service, nil
}