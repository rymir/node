/*
 * Copyright (C) 2017 The "MysteriumNetwork/node" Authors.
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

package session

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/mysterium/_vendor-20180307203655/github.com/cihub/seelog"
	"github.com/mysteriumnetwork/node/identity"
	"github.com/mysteriumnetwork/node/market"
	"github.com/mysteriumnetwork/node/nat/traversal"
)

var (
	// ErrorInvalidProposal is validation error then invalid proposal requested for session creation
	ErrorInvalidProposal = errors.New("proposal does not exist")
	// ErrorSessionNotExists returned when consumer tries to destroy session that does not exists
	ErrorSessionNotExists = errors.New("session does not exists")
	// ErrorWrongSessionOwner returned when consumer tries to destroy session that does not belongs to him
	ErrorWrongSessionOwner = errors.New("wrong session owner")
)

const managerLogPrefix = "[session-manager] "

// IDGenerator defines method for session id generation
type IDGenerator func() (ID, error)

// ConfigNegotiator is able to handle config negotiations
type ConfigNegotiator interface {
	ProvideConfig(consumerKey json.RawMessage) (ServiceConfiguration, DestroyCallback, error)
}

// ConfigProvider provides session config for remote client
type ConfigProvider func(consumerKey json.RawMessage) (ServiceConfiguration, DestroyCallback, error)

// DestroyCallback cleanups session
type DestroyCallback func()

// PromiseProcessor processes promises at provider side.
// Provider checks promises from consumer and signs them also.
// Provider clears promises from consumer.
type PromiseProcessor interface {
	Start(proposal market.ServiceProposal) error
	Stop() error
}

// Storage interface to session storage
type Storage interface {
	Add(sessionInstance Session)
	Find(id ID) (Session, bool)
	Remove(id ID)
}

// BalanceTrackerFactory returns a new instance of balance tracker
type BalanceTrackerFactory func(consumer, provider, issuer identity.Identity) (BalanceTracker, error)

// NATEventGetter lets us access the last known traversal event
type NATEventGetter interface {
	LastEvent() traversal.Event
}

// NewManager returns new session Manager, used by provider
func NewManager(
	currentProposal market.ServiceProposal,
	idGenerator IDGenerator,
	sessionStorage Storage,
	balanceTrackerFactory BalanceTrackerFactory,
	natPingerChan func(json.RawMessage),
	lastSessionShutdown chan struct{},
	natEventGetter NATEventGetter,
	serviceId string,
) *Manager {
	return &Manager{
		currentProposal:       currentProposal,
		generateID:            idGenerator,
		sessionStorage:        sessionStorage,
		balanceTrackerFactory: balanceTrackerFactory,
		natPingerChan:         natPingerChan,
		lastSessionShutdown:   lastSessionShutdown,
		natEventGetter:        natEventGetter,
		serviceId:             serviceId,

		creationLock: sync.Mutex{},
	}
}

// Manager knows how to start and provision session
type Manager struct {
	currentProposal       market.ServiceProposal
	generateID            IDGenerator
	sessionStorage        Storage
	balanceTrackerFactory BalanceTrackerFactory
	provideConfig         ConfigProvider
	natPingerChan         func(json.RawMessage)
	lastSessionShutdown   chan struct{}
	natEventGetter        NATEventGetter
	serviceId             string

	creationLock sync.Mutex
}

// Create creates session instance. Multiple sessions per peerID is possible in case different services are used
func (manager *Manager) Create(consumerID identity.Identity, issuerID identity.Identity, proposalID int, config ServiceConfiguration, requestConfig json.RawMessage) (sessionInstance Session, err error) {
	manager.creationLock.Lock()
	defer manager.creationLock.Unlock()

	if manager.currentProposal.ID != proposalID {
		err = ErrorInvalidProposal
		return
	}

	sessionInstance.ID, err = manager.generateID()
	if err != nil {
		return
	}
	seelog.Infof("TEST: Setting serviceId: %v", manager.serviceId)
	sessionInstance.serviceID = manager.serviceId
	sessionInstance.ConsumerID = consumerID
	sessionInstance.done = make(chan struct{})
	sessionInstance.Config = config
	sessionInstance.CreatedAt = time.Now().UTC()

	balanceTracker, err := manager.balanceTrackerFactory(consumerID, identity.FromAddress(manager.currentProposal.ProviderID), issuerID)
	if err != nil {
		return
	}

	// stop the balance tracker once the session is finished
	go func() {
		<-sessionInstance.done
		balanceTracker.Stop()
	}()

	go func() {
		err := balanceTracker.Start()
		if err != nil {
			log.Error(managerLogPrefix, "balance tracker error: ", err)
			destroyErr := manager.Destroy(consumerID, string(sessionInstance.ID))
			if destroyErr != nil {
				log.Error(managerLogPrefix, "session cleanup failed: ", err)
			}
		}
	}()

	// start NAT pinger here, do not block - configuration should be returned to consumer
	// start NAT pinger, get hole punched, launch service.
	//  on session-destroy - shutdown service and wait for session-create
	// TODO: We might want to start a separate openvpn daemon if node is behind the NAT

	// We need to know that session creation is already in-progress here

	// postpone vpnServer start until NAT hole is punched
	manager.natPingerChan(requestConfig)
	manager.sessionStorage.Add(sessionInstance)
	return sessionInstance, nil
}

// Destroy destroys session by given sessionID
func (manager *Manager) Destroy(consumerID identity.Identity, sessionID string) error {
	manager.creationLock.Lock()
	defer manager.creationLock.Unlock()

	sessionInstance, found := manager.sessionStorage.Find(ID(sessionID))

	if !found {
		return ErrorSessionNotExists
	}

	if sessionInstance.ConsumerID != consumerID {
		return ErrorWrongSessionOwner
	}

	if sessionInstance.Last && manager.lastSessionShutdown != nil {
		log.Info("attempting to stop service")
		if manager.natEventGetter.LastEvent().Type == traversal.FailureEventType {
			log.Info("last session destroy requested - stopping service executable")
			manager.lastSessionShutdown <- struct{}{}
			log.Info("executable shutdown on last session triggered")
		}
	}

	manager.sessionStorage.Remove(ID(sessionID))
	close(sessionInstance.done)

	return nil
}

type ServiceMap struct {
}
