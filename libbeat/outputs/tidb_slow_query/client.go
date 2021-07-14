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
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	// 2000 clusters
	insertStmtCacheSize = 2000
	noTable             = 1146
	noPartition         = 1526
	clusterIDFieldKey   = "kubernetes.namespace"
	noClusterID         = "no_cluster_id"
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

func newClient(
	observer outputs.Observer,
	timeout time.Duration,
	database, dsn string,
	retention, rollStep int,
) (*client, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	cache, err := lru.New(insertStmtCacheSize)
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

// Connect try to create a new connection and replace the conn field
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

	// get table name
	table, err := c.getClusterIDAsTableName(event)
	if err != nil {
		c.observer.Dropped(1)
		batch.Cancelled()
		return err
	}

	// get driver statement
	if !c.stmtCache.Contains(table) {
		sqlString := insertStmt(c.database, table)
		s, err := c.conn.PrepareContext(ctx, sqlString)
		if err != nil {
			newErr := c.handleInsertError(err, ctx, table, event)
			batch.RetryEvents(batch.Events())
			return newErr
		}
		c.stmtCache.Add(table, s)
	}
	sqlStmt, _ := c.stmtCache.Get(table)

	// get placeholder arguments
	fields := getFields(event)

	// execute statement
	_, executionErr := sqlStmt.(*sql.Stmt).ExecContext(ctx, fields...)
	if executionErr != nil {
		newErr := c.handleInsertError(executionErr, ctx, table, event)
		batch.RetryEvents(batch.Events())
		return newErr
	}

	c.observer.NewBatch(1)
	batch.ACK()
	return nil
}

// handle no-table-or-partition error -- create them
// re-throw other unexpected error
func (c *client) handleInsertError(err error, ctx context.Context, table string, event publisher.Event) error {
	if err == nil {
		return nil
	}

	// delete corrupted cached stmt
	c.stmtCache.Remove(table)

	// connection will corrupt when error occurs
	// try to reconnect
	if err := c.Connect(); err != nil {
		return err
	}

	mysqlErr, ok := err.(*mysql.MySQLError)
	if !ok {
		return err
	}

	// create table/partition, could wait for a while
	switch mysqlErr.Number {
	case noPartition:
		return c.createPartitions(ctx, table, event.Content.Timestamp)
	case noTable:
		return c.createTable(ctx, table, event.Content.Timestamp)
	default:
		return mysqlErr
	}
}

func getFields(event publisher.Event) []interface{} {
	r := make([]interface{}, 0, len(orderedColumn))
	for _, c := range orderedColumn {
		// expect nil if fields not exist
		v, _ := event.Content.GetValue(c)
		r = append(r, v)
	}
	return r
}

func (c *client) getClusterIDAsTableName(event publisher.Event) (string, error) {
	v, err := event.Content.GetValue(clusterIDFieldKey)
	if err != nil {
		c.log.Warn("get cluster id as table name failed ", err)
		v = noClusterID
	}
	tableName, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("the value of cluster id must be string")
	}
	return "cluster_" + tableName, nil
}

func (c *client) createTable(ctx context.Context, table string, curTime time.Time) error {
	parts := calculateLessThanPartitionBoundary(curTime, c.rollStep)
	sqlString := createTableStmt(c.database, table, parts)
	_, err := c.conn.ExecContext(ctx, sqlString)
	c.log.Info("create table ", sqlString, "error", err)
	return err
}

func (c *client) createPartitions(ctx context.Context, table string, curTime time.Time) error {
	parts, err := c.getPartitions(ctx, table)
	if err != nil {
		return err
	}
	if len(parts)+c.rollStep > c.retention {
		// drop partitions from head
		dropSqlString := dropPartitionStmt(c.database, table, parts[:c.rollStep])
		_, err := c.conn.ExecContext(ctx, dropSqlString)
		if err != nil {
			return err
		}
	}
	// create new partition at tail
	newParts := calculateLessThanPartitionBoundary(curTime, c.rollStep)
	createSqlString := creationPartitionStmt(c.database, table, newParts)
	_, err = c.conn.ExecContext(ctx, createSqlString)
	c.log.Info("create partitions ", createSqlString, "error", err)
	return err
}

func (c *client) getPartitions(ctx context.Context, table string) ([]string, error) {
	getPartSql := getPartitionStmt(c.database, table)
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
