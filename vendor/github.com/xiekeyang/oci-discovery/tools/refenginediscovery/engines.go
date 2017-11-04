// Copyright 2017 oci-discovery contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package refenginediscovery

import (
	"github.com/sirupsen/logrus"
	"github.com/xiekeyang/oci-discovery/tools/engine"
	"github.com/xiekeyang/oci-discovery/tools/refengine"
	"golang.org/x/net/context"
)

// ResolvedNameCallback templates a callback for use in ResolveName.
type ResolvedNameCallback func(ctx context.Context, root refengine.MerkleRoot, casEngines []engine.Reference) (err error)

// ResolveName iterates over engines calling Engine.RefEngines to get
// potential ref-engine configs.  Then it iterates over those
// ref-engine configs, instantiates a ref engine, and queries that ref
// engine for matching Merkle roots, calling resolvedNameCallback on
// each one.  ResolveName returns any errors returned by
// resolvedNameCallback and aborts further iteration.  Other errors
// (e.g. in initializing a ref engine) generate logged warnings but
// are otherwise ignored.
func ResolveName(ctx context.Context, engines []Engine, name string, resolvedNameCallback ResolvedNameCallback) (err error) {
	for _, engine := range engines {
		err = engine.RefEngines(
			ctx,
			name,
			func(ctx context.Context, refEngine RefEngineReference) (err error) {
				constructor, ok := refengine.Constructors[refEngine.Config.Config.Protocol]
				if !ok {
					logrus.Debugf("unsupported ref-engine protocol %q (%v)", refEngine.Config.Config.Protocol, refengine.Constructors)
					return nil
				}
				eng, err := constructor(ctx, refEngine.Config.URI, refEngine.Config.Config.Data)
				if err != nil {
					logrus.Warnf("failed to initialize %s ref engine with %v: %s", refEngine.Config.Config.Protocol, refEngine.Config.Config.Data, err)
					return nil
				}
				defer eng.Close(ctx)

				roots, err := eng.Get(ctx, name)
				if err != nil {
					logrus.Warn(err)
					return nil
				}
				for _, root := range roots {
					err = resolvedNameCallback(ctx, root, refEngine.CASEngines)
					if err != nil {
						return err
					}
				}

				return nil
			},
		)
		if err != nil {
			return err
		}
	}

	return nil
}
