package db

import "database/sql"

func (d *DB) GetConfig(key string) (string, error) {
	var value string
	err := d.conn.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (d *DB) SetConfig(key, value string) error {
	_, err := d.conn.Exec(
		`INSERT INTO config (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

func (d *DB) GetAllConfig() (map[string]string, error) {
	rows, err := d.conn.Query("SELECT key, value FROM config ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		config[k] = v
	}
	return config, rows.Err()
}
