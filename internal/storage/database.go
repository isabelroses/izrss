// Package storage handles data persistence using SQLite
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/adrg/xdg"
	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// New creates a new database connection at the specified path. WAL mode and a
// busy timeout let the background refresh write without "database is locked".
func New(path string) (*DB, error) {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000"
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Bound the pool and release idle connections so a refresh can't keep a
	// connection open per feed.
	conn.SetMaxOpenConns(8)
	conn.SetMaxIdleConns(2)
	conn.SetConnMaxIdleTime(time.Minute)

	db := &DB{conn: conn}
	if err := db.createTables(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("creating tables: %w", err)
	}

	return db, nil
}

// NewDefault creates a new database connection at the default XDG state location
func NewDefault() (*DB, error) {
	dbPath, err := xdg.StateFile("izrss/izrss.db")
	if err != nil {
		return nil, fmt.Errorf("getting state file path: %w", err)
	}
	return New(dbPath)
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) createTables() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS feed_cache (
			url TEXT PRIMARY KEY,
			content BLOB NOT NULL,
			fetched_at TEXT NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	return db.migrate()
}

// migrations[i] brings the schema from version i to i+1, inside the
// migration's transaction. The schema's version is its PRAGMA user_version:
//
//	0: post_read_status keyed by uuid (the item GUID) alone, so GUID-less
//	   posts shared one row and a GUID in two feeds shared one read flag.
//	1: post_read_status keyed by (feed_url, key), key being rss.Post.Key().
//	2: feed_cache rows say whether they hold the feed's own XML or the
//	   parsed feed, so a start no longer parses every feed.
var migrations = []func(context.Context, *sql.Conn) error{migrateV1, migrateV2}

// schemaVersion is the version of the current schema.
var schemaVersion = len(migrations)

// migrate brings the schema up to schemaVersion. It is a single PRAGMA read
// when the database is already current.
func (db *DB) migrate() error {
	var v int
	if err := db.conn.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	if v >= schemaVersion {
		return nil
	}

	ctx := context.Background()
	conn, err := db.conn.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	// IMMEDIATE takes the write lock up front, so two instances starting at
	// once migrate one after the other rather than racing.
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("beginning migration: %w", err)
	}
	if err := runMigrations(ctx, conn); err != nil {
		_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		return err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("committing migration: %w", err)
	}
	return nil
}

// runMigrations runs the migrations the schema still needs. The version is
// read again under the write lock: another instance may have got here first.
func runMigrations(ctx context.Context, conn *sql.Conn) error {
	var v int
	if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&v); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}
	for ; v < schemaVersion; v++ {
		if err := migrations[v](ctx, conn); err != nil {
			return fmt.Errorf("migrating schema to v%d: %w", v+1, err)
		}
	}
	// PRAGMA takes no bound parameters; v is an int.
	if _, err := conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, v)); err != nil {
		return fmt.Errorf("setting schema version: %w", err)
	}
	return nil
}

// migrateV1 rebuilds post_read_status keyed by (feed_url, key). A v0 row's
// uuid is a GUID, which is exactly Post.Key() for that post, and its feed_url
// is the feed it was first saved from, so rows carry over as they are. The
// empty-uuid row, shared by every GUID-less post, is meaningless and dropped:
// those posts start unread under their new link- or title-based keys.
func migrateV1(ctx context.Context, conn *sql.Conn) error {
	var old int
	if err := conn.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'post_read_status'`).Scan(&old); err != nil {
		return err
	}
	stmts := []string{`
		CREATE TABLE post_read_status_v1 (
			feed_url TEXT NOT NULL,
			key TEXT NOT NULL,
			read INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (feed_url, key)
		)`}
	if old > 0 {
		stmts = append(stmts,
			`INSERT INTO post_read_status_v1 (feed_url, key, read)
				SELECT feed_url, uuid, read FROM post_read_status WHERE uuid <> ''`,
			`DROP TABLE post_read_status`, // and its idx_feed_url; the primary key covers feed_url
		)
	}
	stmts = append(stmts, `ALTER TABLE post_read_status_v1 RENAME TO post_read_status`)
	return execAll(ctx, conn, stmts)
}

// migrateV2 marks every cached feed as the feed's own XML, which is what
// caches before it hold; they are re-saved parsed as they are refreshed.
func migrateV2(ctx context.Context, conn *sql.Conn) error {
	return execAll(ctx, conn, []string{`ALTER TABLE feed_cache ADD COLUMN parsed INTEGER NOT NULL DEFAULT 0`})
}

func execAll(ctx context.Context, conn *sql.Conn, stmts []string) error {
	for _, q := range stmts {
		if _, err := conn.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

// PostReadStatus is one post's read state; Key is rss.Post.Key().
type PostReadStatus struct {
	FeedURL string
	Key     string
	Read    bool
}

// upsertReadStatus saves one post's read status; go-sqlite3 stores bools as
// INTEGER 1 or 0.
const upsertReadStatus = `
	INSERT INTO post_read_status (feed_url, key, read)
	VALUES (?, ?, ?)
	ON CONFLICT(feed_url, key) DO UPDATE SET read = excluded.read
`

// SavePostReadStatuses saves read statuses in a single transaction
func (db *DB) SavePostReadStatuses(statuses []PostReadStatus) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // a no-op once committed

	stmt, err := tx.Prepare(upsertReadStatus)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, status := range statuses {
		if _, err = stmt.Exec(status.FeedURL, status.Key, status.Read); err != nil {
			return fmt.Errorf("saving post %s: %w", status.Key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// LoadPostReadStatuses returns every saved read status
func (db *DB) LoadPostReadStatuses() ([]PostReadStatus, error) {
	rows, err := db.conn.Query(`SELECT feed_url, key, read FROM post_read_status`)
	if err != nil {
		return nil, fmt.Errorf("querying read statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var statuses []PostReadStatus
	for rows.Next() {
		var st PostReadStatus
		if err := rows.Scan(&st.FeedURL, &st.Key, &st.Read); err != nil {
			return nil, fmt.Errorf("scanning row: %w", err)
		}
		statuses = append(statuses, st)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return statuses, nil
}

// CachedFeed is one feed as the cache holds it.
type CachedFeed struct {
	Content []byte
	// Parsed says Content is the parsed feed; otherwise it is the feed's own
	// document, as caches from before schema 2 hold.
	Parsed    bool
	FetchedAt time.Time // zero if unknown
}

// SaveFeedCache stores a parsed feed, fetched now.
func (db *DB) SaveFeedCache(url string, content []byte) error {
	_, err := db.conn.Exec(`
		INSERT INTO feed_cache (url, content, fetched_at, parsed)
		VALUES (?, ?, ?, 1)
		ON CONFLICT(url) DO UPDATE SET content = excluded.content, fetched_at = excluded.fetched_at, parsed = 1
	`, url, content, time.Now().Format(time.RFC3339))
	return err
}

// LoadFeedCache is the cached feed with url; ok is false when none is.
func (db *DB) LoadFeedCache(url string) (c CachedFeed, ok bool, err error) {
	var at string
	err = db.conn.QueryRow(`SELECT content, parsed, fetched_at FROM feed_cache WHERE url = ?`, url).Scan(&c.Content, &c.Parsed, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return CachedFeed{}, false, nil
	}
	if err != nil {
		return CachedFeed{}, false, fmt.Errorf("querying feed cache: %w", err)
	}
	c.FetchedAt, _ = time.Parse(time.RFC3339, at) // an unreadable time is unknown
	return c, true, nil
}
