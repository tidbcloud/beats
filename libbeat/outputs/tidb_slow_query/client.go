package tidb_slow_query

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/outputs"
	"github.com/elastic/beats/v7/libbeat/publisher"
	"github.com/go-sql-driver/mysql"
	lru "github.com/hashicorp/golang-lru"
	"strings"
	"time"
)

const (
	// 2000 clusters
	insertStmtCacheSize = 2000
	// 1146 means no table exists. When a new TiDB cluster is created,
	// generates the first slow log, and filebeat try to insert this log, this error will be triggered.
	// Filebeat will try to create new table for the new cluster.
	mysqlErrCodeTableNotExist = 1146
	// 1526 means no partition. This error occurs when the latest partition boundary < timestamp of incoming slow log.
	// Filebeat will try to create new partition.
	mysqlErrCodePartitionNotExist = 1526
	clusterIDFieldKey             = "kubernetes.namespace"
	noClusterID                   = "NO_CLUSTER_ID"
)

type client struct {
	db        *sql.DB
	conn      *sql.Conn
	observer  outputs.Observer
	timeout   time.Duration
	database  string
	dsn       string
	retention int
	rollStep  int
	stmtCache *lru.Cache
	log       *logp.Logger
}

func newClient(observer outputs.Observer, timeout time.Duration, database, dsn string, retention, rollStep int) (*client, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	cache, err := lru.NewWithEvict(
		insertStmtCacheSize,
		func(_, v interface{}) {
			stmt := v.(*sql.Stmt)
			stmt.Close()
		},
	)
	if err != nil {
		return nil, err
	}
	c := &client{
		observer:  observer,
		timeout:   timeout,
		database:  database,
		db:        db,
		dsn:       dsn,
		retention: retention,
		rollStep:  rollStep,
		stmtCache: cache,
		log:       logp.NewLogger("tidb_slow_query"),
	}
	return c, nil
}

// Connect try to create a new connection and replace the "conn" field
func (c *client) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	if c.conn != nil && c.conn.PingContext(ctx) == nil {
		return nil
	}
	conn, err := c.db.Conn(ctx)
	if err != nil {
		return err
	}
	// Caution! close the broken connection before replacing the reference
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = conn
	return nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) Publish(ct context.Context, batch publisher.Batch) error {
	ctx, cancel := context.WithTimeout(ct, c.timeout)
	defer cancel()

	// todo: support multi events from different cluster (with different table name)
	event := batch.Events()[0]

	clusterID, err := c.extractClusterID(event)
	if err != nil {
		c.observer.Dropped(len(batch.Events()))
		batch.Drop()
		return err
	}
	table := c.buildTableName(clusterID)

	// get driver statement
	if !c.stmtCache.Contains(table) {
		sqlString := buildInsertStmt(c.database, table)
		s, err := c.conn.PrepareContext(ctx, sqlString)
		if err != nil {
			return c.handleMysqlError(err, ctx, table, batch)
		}
		c.stmtCache.Add(table, s)
	}
	sqlStmt, _ := c.stmtCache.Get(table)

	// get placeholder arguments
	fields := convertEventToModel(event)

	// execute statement
	_, executionErr := sqlStmt.(*sql.Stmt).ExecContext(ctx, fields...)
	if executionErr != nil {
		return c.handleMysqlError(executionErr, ctx, table, batch)
	}

	c.observer.NewBatch(len(batch.Events()))
	batch.ACK()
	return nil
}

// handle no-table-or-partition error and do creation
// drop the batch if other errors occurs
func (c *client) handleMysqlError(err error, ctx context.Context, table string, batch publisher.Batch) error {
	if err == nil {
		return nil
	}

	// delete corrupted cached stmt
	c.stmtCache.Remove(table)

	// connection will corrupt when error occurs, try to reconnect
	if err := c.Connect(); err != nil {
		c.observer.Dropped(len(batch.Events()))
		batch.Drop()
		return err
	}

	mysqlErr, ok := err.(*mysql.MySQLError)
	if !ok {
		// drop if not a mysql error
		c.observer.Dropped(len(batch.Events()))
		batch.Drop()
		return err
	}

	switch mysqlErr.Number {
	// create table/partition, could wait for a while
	case mysqlErrCodePartitionNotExist:
		err = c.createPartitions(ctx, table, batch.Events()[0].Content.Timestamp)
		batch.RetryEvents(batch.Events())
	case mysqlErrCodeTableNotExist:
		err = c.createTable(ctx, table, batch.Events()[0].Content.Timestamp)
		batch.RetryEvents(batch.Events())
	default:
		// drop if other error numbers
		c.observer.Dropped(len(batch.Events()))
		batch.Drop()
	}

	return mysqlErr
}

func convertEventToModel(event publisher.Event) []interface{} {
	r := make([]interface{}, 0, len(orderedColumn))
	for _, c := range orderedColumn {
		// expect nil if fields not exist
		v, _ := event.Content.GetValue(c)
		// fixme: there is and inconsistency between slow log and CLUSTER_SLOW_QUERY table schema
		// fixme: "User@Host" in logs   ---->    "User" in table
		// fixme: hard code here
		if c == "User" && v == nil {
			v, _ = event.Content.GetValue("User@Host")
		}
		// ensure safety for string length
		if s, ok := v.(string); ok {
			if l, ok := maxLength[c]; ok && len(s) >= l {
				v = s[:l]
			}
		}

		r = append(r, v)
	}
	return r
}

func (c *client) extractClusterID(event publisher.Event) (string, error) {
	v, err := event.Content.GetValue(clusterIDFieldKey)
	if err != nil {
		c.log.Warnf("get cluster id as table name failed: %s, ", err)
		v = noClusterID
	}
	clusterID, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("the value of cluster id must be string")
	}
	return clusterID, nil
}

func (c *client) buildTableName(clusterID string) string {
	// ensure format of table name is consistent with k8s namespace, "tidb[clusterID]"
	if strings.HasPrefix(clusterID, "tidb") {
		return clusterID
	}
	return fmt.Sprintf("tidb%s", clusterID)
}

func (c *client) createTable(ctx context.Context, table string, curTime time.Time) error {
	parts := calculateLessThanPartitionBoundary(curTime, c.rollStep)
	sqlString := buildCreateTableStmt(c.database, table, parts)
	_, err := c.conn.ExecContext(ctx, sqlString)
	if err != nil {
		return err
	}
	tiFlashSqlString := buildEnableTiFlashStmt(c.database, table)
	_, err = c.conn.ExecContext(ctx, tiFlashSqlString)
	if err != nil {
		return err
	}
	c.log.Infof("created table: %s, enable tiflash: %s", sqlString, tiFlashSqlString)
	return nil
}

func (c *client) createPartitions(ctx context.Context, table string, curTime time.Time) error {
	parts, err := c.getPartitions(ctx, table)
	if err != nil {
		return err
	}
	if len(parts)+c.rollStep > c.retention {
		// drop partitions from head
		dropSqlString := buildDropPartitionStmt(c.database, table, parts[:c.rollStep])
		_, err := c.conn.ExecContext(ctx, dropSqlString)
		if err != nil {
			return err
		}
	}
	// create new partition at tail
	newParts := calculateLessThanPartitionBoundary(curTime, c.rollStep)
	createSqlString := buildCreationPartitionStmt(c.database, table, newParts)
	_, err = c.conn.ExecContext(ctx, createSqlString)
	c.log.Info("create partitions ", createSqlString, "error", err)
	return err
}

func (c *client) getPartitions(ctx context.Context, table string) ([]string, error) {
	getPartSql := buildGetPartitionStmt(c.database, table)
	rows, err := c.conn.QueryContext(ctx, getPartSql)
	if err != nil {
		return nil, err
	}
	parts := make([]string, 0, c.retention)
	for rows.Next() {
		var partName string
		if err := rows.Scan(&partName); err != nil {
			// unexpected error, won't retry
			return nil, err
		}
		parts = append(parts, partName)
	}
	return parts, nil
}

func (c *client) String() string {
	return "mysql(" + c.dsn + ")"
}
