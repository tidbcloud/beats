package convert

/*
A slow query raw log:

# Time: 2021-05-18T14:34:03.62477988Z
# Txn_start_ts: 425026766397767689
# Query_time: 0.002647249
# Parse_time: 0
# Compile_time: 0.000198786
# Rewrite_time: 0.000044676
# Optimize_time: 0.000089838
# Wait_TS: 0.000987767
# Cop_time: 0.001307822 Request_count: 1 Total_keys: 1 Rocksdb_block_cache_hit_count: 1
# Index_names: [stats_fm_sketch:tbl]
# Is_internal: true
# Digest: 4e9ea14d0398e6e6cd86cb8a013d5dcec420bfe697bfc4536e91bdd8a0e26522
# Stats: stats_fm_sketch:pseudo
# Num_cop_tasks: 1
# Cop_proc_avg: 0 Cop_proc_addr: basic-tikv-0.basic-tikv-peer.xuyifan02.svc:20160
# Cop_wait_avg: 0 Cop_wait_addr: basic-tikv-0.basic-tikv-peer.xuyifan02.svc:20160
# Mem_max: 1940
# Prepared: false
# Plan_from_cache: false
# Plan_from_binding: false
# Has_more_results: false
# KV_total: 0.001298434
# PD_total: 0.000965293
# Backoff_total: 0
# Write_sql_response_total: 0
# Succ: true
# Plan: tidb_decode_plan('9wXwZTAJM180CTAJMC4wMAlteXNxbC5zdGF0c19mbV9za2V0Y2gudmFsdWUJMAl0aW1lOjEuMzltcywgbG9vcHM6MSwgQ29uY3VycmVuY3k6T0ZGCTEuNzAgS0IJTi9BCjEJMzBfMTAJMAlfAAleRABMIHRhYmxlX3Rhc2s6IHt0b3RhbF8FbgwgNi43BW8obnVtOiAwLCBjb24VbjQgNX0JMTk2IEJ5dGVzCQFwIDIJNDdfOAkxXw3QAHQBVwA6OtAALCwgaW5kZXg6dGJsKAUhHF9pZCwgaXNfBRdkLCBoaXN0X2lkKSwgcmFuZ2U6WzUzIDAgMiwJB1BdLCBrZWVwIG9yZGVyOmZhbHNlLCAFYhg6cHNldWRvHeUEMm0uKQEIY29wEeIFziwxLCBtYXg6IDEuMzEBKSBwcm9jX2tleXMF6QxycGNfEScBDCkPCDEuMwErgGNvcHJfY2FjaGVfaGl0X3JhdGlvOiAwLjAwfSwgdGlrdglpAHsFNQAwGYU0fSwgc2Nhbl9kZXRhaWw1awF6CGVzcxl9KYIJjIAxLCByb2Nrc2RiOiB7ZGVsZXRlX3NraXBwZWRfY291bnQFrwhrZXlKFgAMYmxvYyHSGasNMgFVBGVhLkEABQ8YYnl0ZTogMCnSGH19fQlOL0EBBCHZDDVfOQl+2QGCmgEgCU4vQQlOL0EK')
# Plan_digest: 3e29e883af27ea7b2a5c334780d94fa5388c49b720825ff2dfdcb5cb0813dce6
select value from mysql.stats_fm_sketch where table_id = 53 and is_index = 0 and hist_id = 2;

Filebeat collects this log via container stdout:
- merge multiline
- add optional k8s meta info
- passe it to tidb_slow_query processor.

The message object received by tidb_slow_query processor looks like:

{
  "@timestamp": "2021-06-30T14:22:31.634Z",
  "@metadata": {
    "beat": "filebeat",
    "type": "_doc",
    "version": "7.13.2"
  },
  "ecs": {
    "version": "1.8.0"
  },
  "host": {
    "name": "2fb52419188e"
  },
  "agent": {
    "version": "7.13.2",
    "hostname": "2fb52419188e",
    "ephemeral_id": "ab60a301-ff61-464d-8865-4dba23b20d55",
    "id": "164c6444-c179-4042-a609-5a492d1ad101",
    "name": "2fb52419188e",
    "type": "filebeat"
  },
  "message": "# Time: 2021-05-18T14:34:03.62477988Z\n# Txn_start_ts: 425026766397767689\n# Query_time: 0.002647249\n# Parse_time: 0\n# Compile_time: 0.000198786\n# Rewrite_time: 0.000044676\n# Optimize_time: 0.000089838\n# Wait_TS: 0.000987767\n# Cop_time: 0.001307822 Request_count: 1 Total_keys: 1 Rocksdb_block_cache_hit_count: 1\n# Index_names: [stats_fm_sketch:tbl]\n# Is_internal: true\n# Digest: 4e9ea14d0398e6e6cd86cb8a013d5dcec420bfe697bfc4536e91bdd8a0e26522\n# Stats: stats_fm_sketch:pseudo\n# Num_cop_tasks: 1\n# Cop_proc_avg: 0 Cop_proc_addr: basic-tikv-0.basic-tikv-peer.xuyifan02.svc:20160\n# Cop_wait_avg: 0 Cop_wait_addr: basic-tikv-0.basic-tikv-peer.xuyifan02.svc:20160\n# Mem_max: 1940\n# Prepared: false\n# Plan_from_cache: false\n# Plan_from_binding: false\n# Has_more_results: false\n# KV_total: 0.001298434\n# PD_total: 0.000965293\n# Backoff_total: 0\n# Write_sql_response_total: 0\n# Succ: true\n# Plan: tidb_decode_plan('9wXwZTAJM180CTAJMC4wMAlteXNxbC5zdGF0c19mbV9za2V0Y2gudmFsdWUJMAl0aW1lOjEuMzltcywgbG9vcHM6MSwgQ29uY3VycmVuY3k6T0ZGCTEuNzAgS0IJTi9BCjEJMzBfMTAJMAlfAAleRABMIHRhYmxlX3Rhc2s6IHt0b3RhbF8FbgwgNi43BW8obnVtOiAwLCBjb24VbjQgNX0JMTk2IEJ5dGVzCQFwIDIJNDdfOAkxXw3QAHQBVwA6OtAALCwgaW5kZXg6dGJsKAUhHF9pZCwgaXNfBRdkLCBoaXN0X2lkKSwgcmFuZ2U6WzUzIDAgMiwJB1BdLCBrZWVwIG9yZGVyOmZhbHNlLCAFYhg6cHNldWRvHeUEMm0uKQEIY29wEeIFziwxLCBtYXg6IDEuMzEBKSBwcm9jX2tleXMF6QxycGNfEScBDCkPCDEuMwErgGNvcHJfY2FjaGVfaGl0X3JhdGlvOiAwLjAwfSwgdGlrdglpAHsFNQAwGYU0fSwgc2Nhbl9kZXRhaWw1awF6CGVzcxl9KYIJjIAxLCByb2Nrc2RiOiB7ZGVsZXRlX3NraXBwZWRfY291bnQFrwhrZXlKFgAMYmxvYyHSGasNMgFVBGVhLkEABQ8YYnl0ZTogMCnSGH19fQlOL0EBBCHZDDVfOQl+2QGCmgEgCU4vQQlOL0EK')\n# Plan_digest: 3e29e883af27ea7b2a5c334780d94fa5388c49b720825ff2dfdcb5cb0813dce6\nselect value from mysql.stats_fm_sketch where table_id = 53 and is_index = 0 and hist_id = 2;",
  "log": {
    "offset": 0,
    "file": {
      "path": ""
    },
    "flags": [
      "multiline"
    ]
  },
  "input": {
    "type": "stdin"
  },
  "container": { ... },
  "kubernetes": { ... }
}

The tidb_slow_query processor parses "message" to k-v pair and add those pairs to the root object. It also check and cast each field type.

*/
