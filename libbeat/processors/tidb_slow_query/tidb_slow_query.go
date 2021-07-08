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

package convert

import (
	"encoding/json"
	"github.com/pkg/errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/processors"
)

const (
	logName  = "processor.tidb_slow_query"
	maxNumKV = 100
)

var kvPat = regexp.MustCompile(`(\S+): (\S+)`)

func init() {
	processors.RegisterPlugin("tidb_slow_query", New)
}

type processor struct {
	config
	log *logp.Logger
}

// New constructs a new tidb_slow_query processor.
func New(cfg *common.Config) (processors.Processor, error) {
	c := defaultConfig()
	if err := cfg.Unpack(&c); err != nil {
		return nil, errors.Wrap(err, "fail to unpack the tidb_slow_query processor configuration")
	}

	return newSlowQuery(c)
}

func newSlowQuery(c config) (*processor, error) {
	log := logp.NewLogger(logName)
	return &processor{config: c, log: log}, nil
}

func (p *processor) String() string {
	jsonConfig, _ := json.Marshal(p.config)
	return "tidb_slow_query=" + string(jsonConfig)
}

func (p *processor) Run(event *beat.Event) (*beat.Event, error) {

	m0, err := event.Fields.GetValue("message")
	if err != nil {
		return nil, err
	}
	m1 := m0.(string)
	p.log.Debug("raw message", m1)

	lines := strings.Split(m1, "\\n")
	p.log.Debug("split lines", lines)

	if len(lines) < 3 {
		return nil, errors.Errorf("slow query log must contain Time and Statement lines: %v", lines)
	}

	// parse k-v
	extractedKV := common.MapStr(make(map[string]interface{}, maxNumKV))
	for i := 0; i < len(lines)-1; i++ {
		matches := kvPat.FindAllStringSubmatch(lines[i], -1)
		p.log.Debug("each line matched and captured", matches)
		for _, match := range matches {
			if len(match) != 3 {
				return nil, errors.Errorf("failed to extract kv for single match: %v", match)
			}
			k, v := match[1], match[2]
			p.log.Debug("each k", k, "each v", v)
			if len(k) > 0 && len(v) > 0 {
				num, err := strconv.ParseFloat(v, 64)
				if err == nil {
					extractedKV.Put(k, num)
				} else {
					extractedKV.Put(k, v)
				}
			}
		}
	}
	event.Fields.Update(extractedKV)
	p.log.Debug("extracted K-Vs", extractedKV.StringToPrint())

	// extract the last line as Statement
	event.PutValue("Statement", lines[len(lines)-1])

	// extract timestamp
	t0, err := event.GetValue("Time")
	if err != nil {
		return nil, err
	}
	t1, err := time.Parse(time.RFC3339Nano, t0.(string))
	if err != nil {
		return nil, err
	}
	event.Timestamp = t1
	event.PutValue("Time", t1)
	p.log.Debug("extracted timestamp", t1)

	// decode plan
	p0, err := extractedKV.GetValue("Plan")
	if err != nil {
		return nil, err
	}
	p1, err := DecodePlan(p0.(string))
	if err != nil {
		return nil, err
	}
	event.PutValue("Plan", p1)
	p.log.Debug("decode plan", p1)

	event.Delete("message")
	p.log.Debug("final event", event)

	return event, nil
}
