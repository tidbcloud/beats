package tidb_slow_query

import (
	"fmt"
	"strings"
	"time"
)

var (
	// reference: executor/slowQueryTuple
	// column_name: column_type
	schemaColumnTypes = map[string]string{
		"Instance":                      "varchar(64)",
		"Time":                          "timestamp(6)",
		"Txn_start_ts":                  "bigint(20) unsigned",
		"User":                          "varchar(64)",
		"Host":                          "varchar(64)",
		"Conn_ID":                       "bigint(20) unsigned",
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
	orderedColumn = make([]string, 0, len(schemaColumnTypes))
	zone, _       = time.LoadLocation("UTC")
)

func init() {
	for k := range schemaColumnTypes {
		orderedColumn = append(orderedColumn, k)
	}
}

func buildInsertStmt(schema, table string) string {
	cols := make([]string, 0, len(orderedColumn))
	args := make([]string, 0, len(orderedColumn))
	for _, c := range orderedColumn {
		cols = append(cols, quoteSchemaObjectIdentifier(c))
		args = append(args, "?")
	}
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("INSERT INTO %s.%s (", quoteSchemaObjectIdentifier(schema), quoteSchemaObjectIdentifier(table)))
	b.WriteString(strings.Join(cols, ","))
	b.WriteString(") ")
	b.WriteString("VALUES (")
	b.WriteString(strings.Join(args, ","))
	b.WriteString(") ")
	b.WriteString(";")
	return b.String()
}

func buildCreateTableStmt(schema, table string, lessThanPartitions []time.Time) string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (", quoteSchemaObjectIdentifier(schema), quoteSchemaObjectIdentifier(table)))
	b.WriteString("`id` bigint(20) unsigned not null AUTO_INCREMENT,")
	for k, v := range schemaColumnTypes {
		b.WriteString(fmt.Sprintf("%s %s,", quoteSchemaObjectIdentifier(k), v))
	}
	b.WriteString("PRIMARY KEY (`id`,`Time`),")
	b.WriteString("INDEX `query_index` (`Digest`, `Conn_ID`)")
	b.WriteString(") ")
	b.WriteString("PARTITION BY RANGE (FLOOR(UNIX_TIMESTAMP(`Time`))) (")
	for i := 0; i < len(lessThanPartitions)-1; i++ {
		p := lessThanPartitions[i]
		unix := p.Unix()
		b.WriteString(fmt.Sprintf("PARTITION %s VALUES LESS THAN (%d),", buildPartitionName(p), unix))
	}
	// add the last partition
	if len(lessThanPartitions) > 0 {
		lastP := lessThanPartitions[len(lessThanPartitions)-1]
		unix := lastP.Unix()
		b.WriteString(fmt.Sprintf("PARTITION %s VALUES LESS THAN (%d)", buildPartitionName(lastP), unix))
	}
	b.WriteString(");")
	return b.String()
}

func buildEnableTiFlashStmt(schema, table string) string {
	return fmt.Sprintf("ALTER TABLE `%s`.`%s` SET TIFLASH REPLICA 1;", schema, table)
}

func calculateLessThanPartitionBoundary(t time.Time, step int) []time.Time {
	parts := make([]time.Time, 0, step)
	// ensure start of the day at +8:00 timezone
	y, m, d := t.In(zone).Date()
	round := time.Date(y, m, d, 0, 0, 0, 0, zone)
	for i := 1; i <= step; i++ {
		parts = append(parts, round.Add(time.Duration(i)*24*time.Hour))
	}
	return parts
}

func buildGetPartitionStmt(schema, table string) string {
	return fmt.Sprintf("SELECT `partition_name` FROM `information_schema`.`partitions` "+
		"WHERE table_schema='%s' AND table_name='%s' AND `partition_name` IS NOT NULL order by `partition_name` asc",
		schema, table)
}

func buildCreationPartitionStmt(schema, table string, lessThanPartitions []time.Time) string {
	b := strings.Builder{}

	b.WriteString(fmt.Sprintf("ALTER TABLE %s.%s ADD PARTITION (", quoteSchemaObjectIdentifier(schema), quoteSchemaObjectIdentifier(table)))
	for i := 0; i < len(lessThanPartitions)-1; i++ {
		p := lessThanPartitions[i]
		unix := p.Unix()
		b.WriteString(fmt.Sprintf("PARTITION %s VALUES LESS THAN (%d),", buildPartitionName(p), unix))
	}
	// add the last partition
	if len(lessThanPartitions) > 0 {
		lastP := lessThanPartitions[len(lessThanPartitions)-1]
		b.WriteString(fmt.Sprintf("PARTITION %s VALUES LESS THAN (%d)", buildPartitionName(lastP), lastP.Unix()))
	}
	b.WriteString(");")
	return b.String()
}

func buildDropPartitionStmt(schema, table string, lessThanPartitions []string) string {
	b := strings.Builder{}

	b.WriteString(fmt.Sprintf("ALTER TABLE %s.%s drop PARTITION ", quoteSchemaObjectIdentifier(schema), quoteSchemaObjectIdentifier(table)))
	for i := 0; i < len(lessThanPartitions)-1; i++ {
		p := lessThanPartitions[i]
		b.WriteString(quoteSchemaObjectIdentifier(p) + ",")
	}
	// add the last partition
	if len(lessThanPartitions) > 0 {
		lastP := lessThanPartitions[len(lessThanPartitions)-1]
		b.WriteString(quoteSchemaObjectIdentifier(lastP))
	}
	b.WriteString(";")
	return b.String()
}

func buildPartitionName(t time.Time) string {
	date := t.In(zone).Format("2006-01-02")
	return quoteSchemaObjectIdentifier("p" + date)
}

func quoteSchemaObjectIdentifier(word string) string {
	return fmt.Sprintf("`%s`", word)
}
