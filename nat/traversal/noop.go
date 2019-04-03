/*
 * Copyright (C) 2019 The "MysteriumNetwork/node" Authors.
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

package traversal

import "encoding/json"

// NoopPinger does nothing
type NoopPinger struct{}

// Start does nothing
func (np *NoopPinger) Start() {}

// Stop does nothing
func (np *NoopPinger) Stop() {}

// PingProvider does nothing
func (np *NoopPinger) PingProvider(ip string, port int) error { return nil }

// PingTarget does nothing
func (np *NoopPinger) PingTarget(json.RawMessage) {}

// BindPort does nothing
func (np *NoopPinger) BindPort(port int) {}

// BindServicePort does nothing
func (np *NoopPinger) BindServicePort(port int) {}

// WaitForHole does nothing
func (np *NoopPinger) WaitForHole() error { return nil }

// NoopEventsTracker does nothing
type NoopEventsTracker struct{}

// ConsumeNATEvent does nothing
func (net *NoopEventsTracker) ConsumeNATEvent(event Event) {}

// LastEvent does nothing
func (net *NoopEventsTracker) LastEvent() Event { return Event{} }

// WaitForEvent does nothing
func (net *NoopEventsTracker) WaitForEvent() Event { return Event{} }
