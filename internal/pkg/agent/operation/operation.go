// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package operation

import (
	"context"

	"github.com/elastic/elastic-agent-client/v7/pkg/proto"

	"github.com/elastic/elastic-agent/internal/pkg/agent/errors"
	"github.com/elastic/elastic-agent/internal/pkg/agent/program"
	"github.com/elastic/elastic-agent/internal/pkg/core/app"
	"github.com/elastic/elastic-agent/internal/pkg/core/monitoring"
	"github.com/elastic/elastic-agent/internal/pkg/core/server"
	"github.com/elastic/elastic-agent/internal/pkg/core/state"
)

// operation is an operation definition
// each operation needs to implement this interface in order
// to ease up rollbacks
type operation interface {
	// Name is human readable name which identifies an operation
	Name() string
	// Check  checks whether operation needs to be run
	// In case prerequisites (such as invalid cert or tweaked binary) are not met, it returns error
	// examples:
	// - Start does not need to run if process is running
	// - Fetch does not need to run if package is already present
	Check(ctx context.Context, application Application) (bool, error)
	// Run runs the operation
	Run(ctx context.Context, application Application) error
}

// Application is an application capable of being started, stopped and configured.
type Application interface {
	Name() string
	Started() bool
	Start(ctx context.Context, p app.Taggable, cfg map[string]interface{}) error
	Stop()
	Shutdown()
	Configure(ctx context.Context, config map[string]interface{}) error
	Monitor() monitoring.Monitor
	State() state.State
	Spec() program.Spec
	SetState(status state.Status, msg string, payload map[string]interface{})
	OnStatusChange(s *server.ApplicationState, status proto.StateObserved_Status, msg string, payload map[string]interface{})
}

// Descriptor defines a program which needs to be run.
// Is passed around operator operations.
type Descriptor interface {
	Spec() program.Spec
	ServicePort() int
	BinaryName() string
	ArtifactName() string
	Version() string
	ID() string
	Directory() string
	Tags() map[app.Tag]string
}

// ApplicationStatusHandler expects that only Application is registered in the server and updates the
// current state of the application from the OnStatusChange callback from inside the server.
//
// In the case that an application is reported as failed by the server it will then restart the application, unless
// it expects that the application should be stopping.
type ApplicationStatusHandler struct{}

// OnStatusChange is the handler called by the GRPC server code.
//
// It updates the status of the application and handles restarting the application is needed.
func (*ApplicationStatusHandler) OnStatusChange(s *server.ApplicationState, status proto.StateObserved_Status, msg string, payload map[string]interface{}) {
	if state.IsStateFiltered(msg, payload) {
		return
	}
	app, ok := s.App().(Application)

	if !ok {
		panic(errors.New("only Application can be registered when using the ApplicationStatusHandler", errors.TypeUnexpected))
	}
	app.OnStatusChange(s, status, msg, payload)
}