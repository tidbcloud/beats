package tidb_slow_query

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

var (
	// reference: executor/slowQueryTuple
	// column_name: column_type
	slowQuerySQLType = map[string]string{
		"Time":                          "timestamp(6)",
		"Txn_start_ts":                  "double",
		"User":                          "varchar(64)",
		"Host":                          "varchar(64)",
		"Conn_ID":                       "double",
		"Exec_retry_count":              "double",
		"Exec_retry_time":               "double",
		"Query_time":                    "double",
		"Parse_time":                    "double",
		"Compile_time":                  "double",
		"Rewrite_time":                  "double",
		"Preproc_subqueries":            "double",
		"Preproc_subqueries_time":       "double",
		"Optimize_time":                 "double",
		"Wait_TS":                       "double",
		"Prewrite_time":                 "double",
		"Wait_prewrite_binlog_time":     "double",
		"Commit_time":                   "double",
		"Get_commit_ts_time":            "double",
		"Commit_backoff_time":           "double",
		"Backoff_types":                 "varchar(64)",
		"Resolve_lock_time":             "double",
		"Local_latch_wait_time":         "double",
		"Write_keys":                    "double",
		"Write_size":                    "double",
		"Prewrite_region":               "double",
		"Txn_retry":                     "double",
		"Cop_time":                      "double",
		"Process_time":                  "double",
		"Wait_time":                     "double",
		"Backoff_time":                  "double",
		"LockKeys_time":                 "double",
		"Request_count":                 "double",
		"Total_keys":                    "double",
		"Process_keys":                  "double",
		"DB":                            "varchar(64)",
		"Index_names":                   "varchar(128)",
		"Digest":                        "varchar(64)",
		"Stats":                         "varchar(512)",
		"Cop_proc_avg":                  "double",
		"Cop_proc_p90":                  "double",
		"Cop_proc_max":                  "double",
		"Cop_proc_addr":                 "varchar(64)",
		"Cop_wait_avg":                  "double",
		"Cop_wait_p90":                  "double",
		"Cop_wait_max":                  "double",
		"Cop_wait_addr":                 "varchar(64)",
		"Mem_max":                       "double",
		"Disk_max":                      "double",
		"Prev_stmt":                     "longtext",
		"Query":                         "longtext",
		"Is_internal":                   "tinyint(1)",
		"Succ":                          "tinyint(1)",
		"Plan_from_cache":               "tinyint(1)",
		"Plan_from_binding":             "tinyint(1)",
		"Prepared":                      "tinyint(1)",
		"KV_total":                      "double",
		"PD_total":                      "double",
		"Backoff_total":                 "double",
		"Write_sql_response_total":      "double",
		"Plan":                          "longtext",
		"Plan_digest":                   "varchar(128)",
		"Backoff_Detail":                "varchar(4096)",
		"Rocksdb_delete_skipped_count":  "double",
		"Rocksdb_key_skipped_count":     "double",
		"Rocksdb_block_cache_hit_count": "double",
		"Rocksdb_block_read_count":      "double",
		"Rocksdb_block_read_byte":       "double",
	}
	zone, _ = time.LoadLocation("Asia/Shanghai")
)

const dateFormat = "2006-01-02"

func insertStmt(schema, table string) string {
	cols := make([]string, 0, len(slowQuerySQLType))
	args := make([]string, 0, len(slowQuerySQLType))
	for k := range slowQuerySQLType {
		cols = append(cols, k)
		args = append(args, "?")
	}
	buf := new(bytes.Buffer)
	buf.WriteString(fmt.Sprintf("INSERT INTO %s.%s (", quote(schema), quote(table)))
	buf.WriteString(strings.Join(cols, ","))
	buf.WriteString(") ")
	buf.WriteString("VALUES (")
	buf.WriteString(strings.Join(args, ","))
	buf.WriteString(") ")
	buf.WriteString(";")
	return buf.String()
}

// add an extra auto_random id column
func createTableStmt(schema, table string, lessThanPartitions ...time.Time) string {
	buf := new(bytes.Buffer)
	buf.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (", quote(schema), quote(table)))
	buf.WriteString("`id` bigint(20) unsigned not null AUTO_RANDOM,")
	for k, v := range slowQuerySQLType {
		buf.WriteString(fmt.Sprintf("%s %s,", k, v))
	}
	buf.WriteString("PRIMARY KEY (`id`)")
	buf.WriteString(") ")
	buf.WriteString("PARTITION BY RANGE (UNIX_TIMESTAMP(`Time`)) (")
	for _, p := range lessThanPartitions {
		unix := p.Unix()
		buf.WriteString(fmt.Sprintf("PARTITION %s VALUES LESS THAN (%d),", partName(p), unix))
	}
	// delete the last ,
	if len(lessThanPartitions) > 0 {
		buf.Truncate(buf.Len() - 1)
	}
	buf.WriteString(");")
	return buf.String()
}

func getPartitionStmt(schema, table string) string {
	return fmt.Sprintf("SELECT `partition_name` FROM `information_schema`.`partitions` "+
		"WHERE table_schema=%s AND table_name=%s AND `partition_name` IS NOT NULL order by `partition_name` asc",
		quote(schema), quote(table))
}

func creationPartitionStmt(schema, table string, lessThanPartitions []time.Time) string {
	buf := new(bytes.Buffer)

	buf.WriteString(fmt.Sprintf("ALTER TABLE %s.%s ADD PARTITION (", quote(schema), quote(table)))
	for _, p := range lessThanPartitions {
		unix := p.Unix()
		buf.WriteString(fmt.Sprintf("PARTITION %s VALUES LESS THAN (%d),", partName(p), unix))
	}
	// delete the last ,
	if len(lessThanPartitions) > 0 {
		buf.Truncate(buf.Len() - 1)
	}
	buf.WriteString(");")
	return buf.String()
}

func dropPartitionStmt(schema, table string, lessThanPartitions []string) string {
	buf := new(bytes.Buffer)

	buf.WriteString(fmt.Sprintf("ALTER TABLE %s.%s drop PARTITION ", quote(schema), quote(table)))
	for _, p := range lessThanPartitions {
		buf.WriteString(p + ",")
	}
	// delete the last ,
	if len(lessThanPartitions) > 0 {
		buf.Truncate(buf.Len() - 1)
	}
	buf.WriteString(";")
	return buf.String()
}

func partName(t time.Time) string {
	date := t.In(zone).Format(dateFormat)
	return "p" + date
}

func quote(word string) string {
	return "`" + word + "`"
}
