package db

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
)

func floatsToBytes(values []float32) []byte {
	if len(values) == 0 {
		return nil
	}
	buf := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}
func bytesToFloats(data []byte) []float32 {
	if len(data) == 0 {
		return nil
	}
	out := make([]float32, len(data)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return out
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func sortSearch(items []SearchHit) {
	sort.Slice(items, func(i, j int) bool { return items[i].Final > items[j].Final })
}
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func ensureColumn(
	db *sql.DB,
	table string,
	column string,
	definition string,
) error {
	rows, err := db.Query(
		fmt.Sprintf("PRAGMA table_info(%s)", table),
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			dataType   string
			notNull    int
			defaultV   any
			primaryKey int
		)
		if err := rows.Scan(
			&cid,
			&name,
			&dataType,
			&notNull,
			&defaultV,
			&primaryKey,
		); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(fmt.Sprintf(
		"ALTER TABLE %s ADD COLUMN %s %s",
		table,
		column,
		definition,
	))
	return err
}
