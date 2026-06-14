package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBError struct {
	Op      string
	Code    int
	Message string
	Err     error
}

func (e *DBError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Op, e.Message)
}

func (e *DBError) Unwrap() error {
	return e.Err
}

var DB *pgxpool.Pool

var connStr = "postgres://pic_admin:super_secret_password_123@localhost:5432/pic_shortener_db?sslmode=disable"

func InitDBSQLite(ctx context.Context, user, password, host, name string) (*pgxpool.Pool, error) {
	var err error
	InitQuery := `CREATE TABLE IF NOT EXISTS originals (
	id SERIAL PRIMARY KEY,
	hash VARCHAR(64) NOT NULL UNIQUE,
	format VARCHAR(10) NOT NULL,
	ts INTEGER
	);`
	SecondInitQuery := `CREATE TABLE IF NOT EXISTS cashed(
	id SERIAL PRIMARY KEY,
	original_id INTEGER NOT NULL,
	width INTEGER NOT NULL,
	quality INTEGER NOT NULL,
	filepath VARCHAR(256) NOT NULL,
	format VARCHAR(10) NOT NULL,
	last_access_at INTEGER,
	FOREIGN KEY (original_id) REFERENCES originals(id) ON DELETE CASCADE
	);`

	conn := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable", user, password, host, name)

	fmt.Printf("Connection link: %s\n", conn)

	DB, err = pgxpool.New(ctx, conn)
	if err != nil {
		return nil, &DBError{
			Op:      "Creating New Database",
			Code:    400,
			Message: "Might be that you've used incorrect data in .env file",
			Err:     err,
		}
	}

	_, err = DB.Exec(ctx, InitQuery)
	if err != nil {
		return nil, &DBError{
			Op:      "Database Initialization #1",
			Code:    400,
			Message: "ERR at DB exec type your REQ-ID to our support",
			Err:     err,
		}
	}

	_, err = DB.Exec(ctx, SecondInitQuery)
	if err != nil {
		return nil, &DBError{
			Op:      "Database Initialization #2",
			Code:    400,
			Message: "ERR at DB exec type your REQ-ID to our support",
			Err:     err,
		}
	}

	return DB, nil
}

func SaveOriginalAfterPostRequest(ctx context.Context, hash, format string, ts int) error {
	OriginalQuery := `INSERT INTO originals(hash, format, ts) VALUES($1, $2, $3);`

	_, err := DB.Exec(ctx, OriginalQuery, hash, format, ts)
	if err != nil {
		return &DBError{
			Op:      "Database Exec After Post Req",
			Code:    500,
			Message: "ERR at DB exec type your REQ-ID to our support",
			Err:     err,
		}
	}
	return nil
}

func SaveCashedAfterGetRequest(ctx context.Context, id, width, quality, filepath, format string, last int) error {
	CashQuery := `INSERT INTO cashed(original_id, width, quality, filepath, format, last_access_at) VALUES($1, $2, $3, $4, $5, $6);`

	_, err := DB.Exec(ctx, CashQuery, id, width, quality, filepath, format, last)
	if err != nil {
		return &DBError{
			Op:      "Database Exec After Get Req",
			Code:    500,
			Message: "ERR at DB exec type your REQ-ID to our support",
			Err:     err,
		}
	}
	return nil
}

func GetOriginalData(ctx context.Context, hash string) (int, error) {
	orig_Query := `SELECT id FROM originals WHERE hash = $1;`
	var id int

	row := DB.QueryRow(ctx, orig_Query, hash)
	err := row.Scan(&id)
	if err != nil {
		return -1, &DBError{
			Op:      "Getting data from Query Row",
			Code:    500,
			Message: "ERR at DB exec type your REQ-ID to our support",
			Err:     err,
		}
	}

	return id, nil
}

func UploadMap(ctx context.Context) (map[string]string, error) {

	resultMap := make(map[string]string)

	queryForData := `SELECT o.hash, c.quality, c.width, c.filepath, c.format
	FROM cashed c
	JOIN originals o ON c.original_id = o.id;`

	rows, err := DB.Query(ctx, queryForData)
	if err != nil {
		return nil, &DBError{
			Op:      "Taking rows for filling maps",
			Code:    500,
			Message: "Unexpected error, tell your REQ-ID to our support",
			Err:     err,
		}
	}
	defer rows.Close()

	for rows.Next() {
		var (
			hash     string
			quality  int
			width    int
			format   string
			filepath string
		)

		err = rows.Scan(&hash, &quality, &width, &filepath, &format)
		if err != nil {
			return nil, &DBError{
				Op:      "Scanning Rows",
				Code:    400,
				Message: "ERR at getting info for you, call our support with your REQ-ID",
				Err:     err,
			}
		}

		key := fmt.Sprintf("%s_%v_%v_%v", hash, width, quality, format)

		resultMap[key] = filepath
	}

	return resultMap, nil
}

func GetFormat(ctx context.Context, id string) (string, error) {
	var format string
	Query := `SELECT format FROM cached WHERE id = $1`
	row := DB.QueryRow(ctx, Query, id)

	err := row.Scan(&format)
	if err != nil {
		return "", &DBError{
			Op:      "Taking format",
			Code:    500,
			Message: "Unexpected error, tell your REQ-ID to our support",
			Err:     err,
		}
	}

	return format, nil
}
