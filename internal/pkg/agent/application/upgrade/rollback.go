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

package upgrade

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/elastic/elastic-agent/internal/pkg/agent/application/paths"
	"github.com/elastic/elastic-agent/internal/pkg/agent/control"
	"github.com/elastic/elastic-agent/internal/pkg/agent/control/client"
	"github.com/elastic/elastic-agent/internal/pkg/agent/errors"
	"github.com/elastic/elastic-agent/internal/pkg/agent/install"
	"github.com/elastic/elastic-agent/internal/pkg/core/backoff"
	"github.com/elastic/elastic-agent/internal/pkg/core/logger"
)

const (
	watcherSubcommand  = "watch"
	maxRestartCount    = 5
	restartBackoffInit = 5 * time.Second
	restartBackoffMax  = 90 * time.Second
)

// Rollback rollbacks to previous version which was functioning before upgrade.
func Rollback(ctx context.Context, prevHash, currentHash string) error {
	// change symlink
	if err := ChangeSymlink(ctx, prevHash); err != nil {
		return err
	}

	// revert active commit
	if err := UpdateActiveCommit(prevHash); err != nil {
		return err
	}

	// Restart
	if err := restartAgent(ctx); err != nil {
		return err
	}

	// cleanup everything except version we're rolling back into
	return Cleanup(prevHash, true)
}

// Cleanup removes all artifacts and files related to a specified version.
func Cleanup(currentHash string, removeMarker bool) error {
	<-time.After(afterRestartDelay)

	// remove upgrade marker
	if removeMarker {
		if err := CleanMarker(); err != nil {
			return err
		}
	}

	// remove data/elastic-agent-{hash}
	dataDir, err := os.Open(paths.Data())
	if err != nil {
		return err
	}

	subdirs, err := dataDir.Readdirnames(0)
	if err != nil {
		return err
	}

	// remove symlink to avoid upgrade failures, ignore error
	_ = os.Remove(prevSymlinkPath())

	dirPrefix := fmt.Sprintf("%s-", agentName)
	currentDir := fmt.Sprintf("%s-%s", agentName, currentHash)
	for _, dir := range subdirs {
		if dir == currentDir {
			continue
		}

		if !strings.HasPrefix(dir, dirPrefix) {
			continue
		}

		hashedDir := filepath.Join(paths.Data(), dir)
		if cleanupErr := install.RemovePath(hashedDir); cleanupErr != nil {
			err = multierror.Append(err, cleanupErr)
		}
	}

	return err
}

// InvokeWatcher invokes an agent instance using watcher argument for watching behavior of
// agent during upgrade period.
func InvokeWatcher(log *logger.Logger) error {
	if !IsUpgradeable() {
		log.Debug("agent is not upgradable, not starting watcher")
		return nil
	}

	versionedHome := paths.VersionedHome(paths.Top())
	cmd := invokeCmd(versionedHome)
	defer func() {
		if cmd.Process != nil {
			log.Debugf("releasing watcher %v", cmd.Process.Pid)
			cmd.Process.Release()
		}
	}()

	return cmd.Start()
}

func restartAgent(ctx context.Context) error {
	restartFn := func(ctx context.Context) error {
		c := client.New()
		err := c.Connect(ctx)
		if err != nil {
			return errors.New(err, "failed communicating to running daemon", errors.TypeNetwork, errors.M("socket", control.Address()))
		}
		defer c.Disconnect()

		err = c.Restart(ctx)
		if err != nil {
			return errors.New(err, "failed trigger restart of daemon")
		}

		return nil
	}

	signal := make(chan struct{})
	backExp := backoff.NewExpBackoff(signal, restartBackoffInit, restartBackoffMax)

	for i := maxRestartCount; i >= 1; i-- {
		backExp.Wait()
		err := restartFn(ctx)
		if err == nil {
			break
		}

		if i == 1 {
			return err
		}
	}

	close(signal)
	return nil
}