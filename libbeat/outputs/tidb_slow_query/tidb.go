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

package tidb_slow_query

import (
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/outputs"
)

const batchSize = 1

func init() {
	outputs.RegisterType("tidb_slow_query", makeTiDB)
}

func makeTiDB(
	_ outputs.IndexManager,
	_ beat.Info,
	observer outputs.Observer,
	cfg *common.Config,
) (outputs.Group, error) {
	config := defaultConfig
	err := cfg.Unpack(&config)
	if err != nil {
		return outputs.Fail(err)
	}
	if config.checkMutualTLSEnable() {
		if err := config.RegisterTLS(); err != nil {
			return outputs.Fail(err)
		}
	}
	c, err := newClient(observer, config.Timeout, config.Database, config.DSN(), config.Partition.Retention, config.Partition.RollStep)
	if err != nil {
		return outputs.Fail(err)
	}
	backC := newBackoffClient(c, config.Backoff.Init, config.Backoff.Max)
	return outputs.Success(batchSize, config.MaxRetries, backC)
}
