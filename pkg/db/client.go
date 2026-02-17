package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Client struct {
	conn *sql.DB
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Connect(driver, url string) error {
	db, err := sql.Open(driver, url)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		return err
	}
	c.conn = db
	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Exec(query string, args ...interface{}) (int64, error) {
	if c.conn == nil {
		return 0, fmt.Errorf("database not connected")
	}
	res, err := c.conn.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (c *Client) Query(query string, args ...interface{}) ([]map[string]interface{}, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("database not connected")
	}
	rows, err := c.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, colName := range cols {
			val := columnPointers[i].(*interface{})
			row[colName] = *val
		}
		results = append(results, row)
	}
	return results, nil
}
