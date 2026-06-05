package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDBSQLite(dbName string) (*sql.DB, error) {
	var err error
	InitQuery := `CREATE TABLE IF NOT EXISTS originals (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	hash TEXT UNIQUE NOT NULL,
	filepath TEXT NOT NULL,
	ts INTEGER
	);`
	SecondInitQuery := `CREATE TABLE IF NOT EXISTS cashed(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	original_id INTEGER NOT NULL,
	width INTEGER NOT NULL,
	quality INTEGER NOT NULL,
	filepath TEXT NOT NULL,
	last_access_at INTEGER,
	FOREIGN KEY (original_id) REFERENCES original_id(id) ON DELETE CASCADE
	);`

	DB, err = sql.Open("sqlite", dbName)
	if err != nil {
		return nil, err
	}

	_, err = DB.Exec(InitQuery)
	if err != nil {
		return nil, err
	}

	_, err = DB.Exec(SecondInitQuery)
	if err != nil {
		return nil, err
	}

	return DB, nil
}

func SaveOriginalAfterPostRequest(hash, filepath string, ts int) error {
	OriginalQuery := `INSERT INTO originals(hash, filepath, ts) VALUES(?, ?, ?);`

	_, err := DB.Exec(OriginalQuery, hash, filepath, ts)
	if err != nil {
		log.Printf("Database exec error at saving original image\n")
		return err
	}
	return nil
}

func SaveCashedAfterGetRequest(id, width, quality string, filepath string, last int) error {
	CashQuery := `INSERT INTO cashed(original_id, width, quality, filepath, last_access_at) VALUES(?, ?, ?, ?, ?);`

	stmt, err := DB.Prepare(CashQuery)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(id, width, quality, filepath, last)
	if err != nil {
		return err
	}

	return nil
}

func GetOriginalData(hash string) (int, error) {
	orig_Query := `SELECT id FROM originals WHERE hash = ?;`
	var id int

	row := DB.QueryRow(orig_Query, hash)
	err := row.Scan(&id)
	if err != nil {
		return -1, err
	}

	return id, nil
}

func UploadMap() (map[string]string, error) {

	resultMap := make(map[string]string)

	queryForData := `SELECT o.hash, c.quality, c.width, c.filepath
	FROM cashed c
	JOIN originals o ON c.original_id = o.id;`

	rows, err := DB.Query(queryForData)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			hash     string
			quality  int
			width    int
			filepath string
		)

		err = rows.Scan(&hash, &quality, &width, &filepath)
		if err != nil {
			return nil, err
		}

		key := fmt.Sprintf("%s:%v:%v", hash, quality, width)

		resultMap[key] = filepath
	}

	return resultMap, nil
}

