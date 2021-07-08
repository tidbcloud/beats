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
	"time"
)

type Config struct {
	// connections
	Host     string        `config:"host"`
	Port     int           `config:"port"`
	User     string        `config:"user"`
	Password string        `config:"password"`
	Database string        `config:"database"`
	Timeout  time.Duration `config:"timeout"`

	// retry
	MaxRetries int     `config:"max_retries"`
	Backoff    Backoff `config:"backoff"`

	// sql range partition
	Partition Partition `config:"partition"`
}

type Backoff struct {
	Init time.Duration
	Max  time.Duration
}

type Partition struct {
	Retention int
	RollStep  int
}

var defaultConfig = Config{
	Port:       4000,
	Timeout:    10 * time.Second,
	MaxRetries: 3,
	Backoff: Backoff{
		Init: 1 * time.Second,
		Max:  10 * time.Second,
	},
	Partition: Partition{
		Retention: 365,
		RollStep:  3,
	},
}
