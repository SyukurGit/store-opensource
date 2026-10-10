package repository

import (
	"strconv"

	"storeku/internal/database"
)

type SettingRepository struct {
	db *database.DB
}

func NewSettingRepository(db *database.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

func (r *SettingRepository) Get(key, defaultVal string) string {
	var val string
	err := r.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&val)
	if err != nil || val == "" {
		return defaultVal
	}
	return val
}

func (r *SettingRepository) GetFloat(key string, defaultVal float64) float64 {
	valStr := r.Get(key, "")
	if valStr == "" {
		return defaultVal
	}
	if f, err := strconv.ParseFloat(valStr, 64); err == nil {
		return f
	}
	return defaultVal
}

func (r *SettingRepository) GetInt64(key string, defaultVal int64) int64 {
	valStr := r.Get(key, "")
	if valStr == "" {
		return defaultVal
	}
	if i, err := strconv.ParseInt(valStr, 10, 64); err == nil {
		return i
	}
	return defaultVal
}

func (r *SettingRepository) Set(key, value string) error {
	query := `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`
	_, err := r.db.Exec(query, key, value)
	return err
}

func (r *SettingRepository) GetAll() (map[string]string, error) {
	rows, err := r.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			res[k] = v
		}
	}
	return res, nil
}
