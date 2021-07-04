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
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/common"
)

func TestConvertRun(t *testing.T) {
	tests := map[string]struct {
		config      common.MapStr
		input       beat.Event
		expected    beat.Event
		fail        bool
		errContains string
	}{
		"should success": {
			config: common.MapStr{
				"fields": []common.MapStr{},
			},
			input: beat.Event{
				Fields: common.MapStr{
					"message": "# Time: 2021-05-18T14:34:03.62477988Z\\n# Txn_start_ts: 425026766397767689\\n# Query_time: 0.002647249\\n# Parse_time: 0\\n# Compile_time: 0.000198786\\n# Rewrite_time: 0.000044676\\n# Optimize_time: 0.000089838\\n# Wait_TS: 0.000987767\\n# Cop_time: 0.001307822 Request_count: 1 Total_keys: 1 Rocksdb_block_cache_hit_count: 1\\n# Index_names: [stats_fm_sketch:tbl]\\n# Is_internal: true\\n# Digest: 4e9ea14d0398e6e6cd86cb8a013d5dcec420bfe697bfc4536e91bdd8a0e26522\\n# Stats: stats_fm_sketch:pseudo\\n# Num_cop_tasks: 1\\n# Cop_proc_avg: 0 Cop_proc_addr: basic-tikv-0.basic-tikv-peer.xuyifan02.svc:20160\\n# Cop_wait_avg: 0 Cop_wait_addr: basic-tikv-0.basic-tikv-peer.xuyifan02.svc:20160\\n# Mem_max: 1940\\n# Prepared: false\\n# Plan_from_cache: false\\n# Plan_from_binding: false\\n# Has_more_results: false\\n# KV_total: 0.001298434\\n# PD_total: 0.000965293\\n# Backoff_total: 0\\n# Write_sql_response_total: 0\\n# Succ: true\\n# Plan: tidb_decode_plan('9wXwZTAJM180CTAJMC4wMAlteXNxbC5zdGF0c19mbV9za2V0Y2gudmFsdWUJMAl0aW1lOjEuMzltcywgbG9vcHM6MSwgQ29uY3VycmVuY3k6T0ZGCTEuNzAgS0IJTi9BCjEJMzBfMTAJMAlfAAleRABMIHRhYmxlX3Rhc2s6IHt0b3RhbF8FbgwgNi43BW8obnVtOiAwLCBjb24VbjQgNX0JMTk2IEJ5dGVzCQFwIDIJNDdfOAkxXw3QAHQBVwA6OtAALCwgaW5kZXg6dGJsKAUhHF9pZCwgaXNfBRdkLCBoaXN0X2lkKSwgcmFuZ2U6WzUzIDAgMiwJB1BdLCBrZWVwIG9yZGVyOmZhbHNlLCAFYhg6cHNldWRvHeUEMm0uKQEIY29wEeIFziwxLCBtYXg6IDEuMzEBKSBwcm9jX2tleXMF6QxycGNfEScBDCkPCDEuMwErgGNvcHJfY2FjaGVfaGl0X3JhdGlvOiAwLjAwfSwgdGlrdglpAHsFNQAwGYU0fSwgc2Nhbl9kZXRhaWw1awF6CGVzcxl9KYIJjIAxLCByb2Nrc2RiOiB7ZGVsZXRlX3NraXBwZWRfY291bnQFrwhrZXlKFgAMYmxvYyHSGasNMgFVBGVhLkEABQ8YYnl0ZTogMCnSGH19fQlOL0EBBCHZDDVfOQl+2QGCmgEgCU4vQQlOL0EK')\\n# Plan_digest: 3e29e883af27ea7b2a5c334780d94fa5388c49b720825ff2dfdcb5cb0813dce6\\nselect value from mysql.stats_fm_sketch where table_id = 53 and is_index = 0 and hist_id = 2;",
				},
			},
			expected: beat.Event{
				Fields: common.MapStr{
					"message": "80",
				},
			},
			fail: true,
		},
	}

	for title, tt := range tests {
		t.Run(title, func(t *testing.T) {
			processor, err := New(common.MustNewConfigFrom(tt.config))
			if err != nil {
				t.Fatal(err)
			}
			result, err := processor.Run(&tt.input)
			if tt.expected.Fields != nil {
				assert.Equal(t, tt.expected.Fields.Flatten(), result.Fields.Flatten())
				assert.Equal(t, tt.expected.Meta.Flatten(), result.Meta.Flatten())
				assert.Equal(t, tt.expected.Timestamp, result.Timestamp)
			}
			if tt.fail {
				assert.Error(t, err)
				t.Log("got expected error", err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			assert.NoError(t, err)
		})
	}
}
