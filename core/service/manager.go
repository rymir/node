/*
 * Copyright (C) 2018 The "MysteriumNetwork/node" Authors.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package service

import (
	"encoding/json"
	"errors"

	log "github.com/cihub/seelog"
	"github.com/gofrs/uuid"
	"github.com/mysterium/_vendor-20180307203655/github.com/cihub/seelog"
	"github.com/mysteriumnetwork/node/communication"
	"github.com/mysteriumnetwork/node/identity"
	"github.com/mysteriumnetwork/node/market"
	"github.com/mysteriumnetwork/node/session"
)

// StopTopic is used in event bus to announce that service was stopped
const StopTopic = "Service stop"

var (
	// ErrorLocation error indicates that action (i.e. disconnect)
	ErrorLocation = errors.New("failed to detect service location")
	// ErrUnsupportedServiceType indicates that manager tried to create an unsupported service type
	ErrUnsupportedServiceType = errors.New("unsupported service type")
)

// Service interface represents pluggable Mysterium service
type Service interface {
	Serve(providerID identity.Identity) error
	Stop() error
	ProvideConfig(publicKey json.RawMessage) (session.ServiceConfiguration, session.DestroyCallback, error)
}

// NATPinger defines Pinger interface for Provider
type NATPinger interface {
	BindPort(port int)
	WaitForHole() error
}

// DialogWaiterFactory initiates communication channel which waits for incoming dialogs
type DialogWaiterFactory func(providerID identity.Identity, serviceType string) (communication.DialogWaiter, error)

// DialogHandlerFactory initiates instance which is able to handle incoming dialogs
type DialogHandlerFactory func(market.ServiceProposal, session.ConfigNegotiator, string) communication.DialogHandler

// DiscoveryFactory initiates instance which is able announce service discoverability
type DiscoveryFactory func() Discovery

// Discovery registers the service to the discovery api periodically
type Discovery interface {
	Start(ownIdentity identity.Identity, proposal market.ServiceProposal)
	Stop()
	Wait()
}

// WaitForNATHole blocks until NAT hole is punched towards consumer through local NAT or until hole punching failed
type WaitForNATHole func() error

// NewManager creates new instance of pluggable instances manager
func NewManager(
	serviceRegistry *Registry,
	dialogWaiterFactory DialogWaiterFactory,
	dialogHandlerFactory DialogHandlerFactory,
	discoveryFactory DiscoveryFactory,
	natPinger NATPinger,
	eventPublisher Publisher,
) *Manager {
	return &Manager{
		serviceRegistry:      serviceRegistry,
		servicePool:          NewPool(),
		dialogWaiterFactory:  dialogWaiterFactory,
		dialogHandlerFactory: dialogHandlerFactory,
		discoveryFactory:     discoveryFactory,
		natPinger:            natPinger,
		eventPublisher:       eventPublisher,
	}
}

// Publisher is responsible for publishing given events
type Publisher interface {
	Publish(topic string, args ...interface{})
}

// Manager entrypoint which knows how to start pluggable Mysterium instances
type Manager struct {
	dialogWaiterFactory  DialogWaiterFactory
	dialogHandlerFactory DialogHandlerFactory

	serviceRegistry *Registry
	servicePool     *Pool

	discoveryFactory DiscoveryFactory

	natPinger      NATPinger
	eventPublisher Publisher
}

// Start starts an instance of the given service type if knows one in service registry.
// It passes the options to the start method of the service.
// If an error occurs in the underlying service, the error is then returned.
func (manager *Manager) Start(providerID identity.Identity, serviceType string, options Options) (id ID, err error) {
	service, proposal, err := manager.serviceRegistry.Create(serviceType, options)
	if err != nil {
		return id, err
	}

	dialogWaiter, err := manager.dialogWaiterFactory(providerID, serviceType)
	if err != nil {
		return id, err
	}
	providerContact, err := dialogWaiter.Start()
	if err != nil {
		return id, err
	}
	proposal.SetProviderContact(providerID, providerContact)

	id, err = generateID()
	if err != nil {
		return id, err
	}

	dialogHandler := manager.dialogHandlerFactory(proposal, service, string(id))
	if err = dialogWaiter.ServeDialogs(dialogHandler); err != nil {
		return id, err
	}

	discovery := manager.discoveryFactory()
	discovery.Start(providerID, proposal)

	instance := Instance{
		id:           id,
		state:        Starting,
		options:      options,
		service:      service,
		proposal:     proposal,
		dialogWaiter: dialogWaiter,
		discovery:    discovery,
	}

	manager.servicePool.Add(&instance)

	go func() {
		instance.state = Running
		serveErr := service.Serve(providerID)
		if serveErr != nil {
			log.Error("Service serve failed: ", serveErr)
		}

		instance.state = NotRunning

		seelog.Info("TEST: service.Manager.Start ended serving, stopping")
		// TODO: fix this - Stop is invoked twice
		stopErr := manager.servicePool.Stop(id)
		if stopErr != nil {
			log.Error("Service stop failed: ", stopErr)
		}

		discovery.Wait()
	}()

	return id, nil
}

func generateID() (ID, error) {
	uid, err := uuid.NewV4()
	if err != nil {
		return ID(""), err
	}
	return ID(uid.String()), nil
}

// List returns array of running service instances.
func (manager *Manager) List() map[ID]*Instance {
	return manager.servicePool.List()
}

// Kill stops all services.
func (manager *Manager) Kill() error {
	return manager.servicePool.StopAll()
}

// Stop stops the service.
func (manager *Manager) Stop(id ID) error {
	instance := manager.servicePool.Instance(id)
	if instance == nil {
		return errors.New("service not found")
	}

	seelog.Info("TEST: service.Manager.Stop")
	err := manager.servicePool.Stop(id)
	if err != nil {
		return err
	}

	manager.eventPublisher.Publish(StopTopic, instance)
	return nil
}

// Service returns a service instance by requested id.
func (manager *Manager) Service(id ID) *Instance {
	return manager.servicePool.Instance(id)
}
