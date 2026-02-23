package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// findFirefoxCookies searches the default Firefox profile for claude.ai cookies.
// Returns sessionKey and lastActiveOrg if found.
func findFirefoxCookies() (sessionKey, orgID string, err error) {
	profilesDir, err := findFirefoxProfilesDir()
	if err != nil {
		return "", "", fmt.Errorf("finding Firefox profiles: %w", err)
	}

	profileDir, err := findDefaultProfile(profilesDir)
	if err != nil {
		return "", "", fmt.Errorf("finding default Firefox profile: %w", err)
	}

	log.Println("Firefox profile:", profileDir)

	dbPath := filepath.Join(profileDir, "cookies.sqlite")
	cookies, err := readClaudeAICookies(dbPath)
	if err != nil {
		return "", "", fmt.Errorf("reading Firefox cookies: %w", err)
	}

	sessionKey = cookies["sessionKey"]
	orgID = cookies["lastActiveOrg"]

	if sessionKey == "" {
		return "", "", fmt.Errorf("sessionKey not found — are you logged in to claude.ai in Firefox?")
	}
	if orgID == "" {
		return "", "", fmt.Errorf("lastActiveOrg not found in Firefox cookies")
	}

	log.Printf("Firefox cookies found: org_id=%s...", orgID[:min(8, len(orgID))])
	return sessionKey, orgID, nil
}

// findFirefoxProfilesDir returns the Firefox base directory for the current OS.
func findFirefoxProfilesDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		base = filepath.Join(appData, "Mozilla", "Firefox")
	default: // linux, darwin
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting home directory: %w", err)
		}
		base = filepath.Join(home, ".mozilla", "firefox")
	}
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return "", fmt.Errorf("Firefox directory not found: %s", base)
	}
	return base, nil
}

// findDefaultProfile parses profiles.ini and returns the default profile directory.
// Falls back to the first profile if no Default=1 is set.
func findDefaultProfile(firefoxDir string) (string, error) {
	iniPath := filepath.Join(firefoxDir, "profiles.ini")
	f, err := os.Open(iniPath)
	if err != nil {
		return "", fmt.Errorf("opening profiles.ini: %w", err)
	}
	defer f.Close()

	type entry struct {
		path       string
		isRelative bool
		isDefault  bool
	}

	var profiles []entry
	var cur *entry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[Profile") {
			profiles = append(profiles, entry{})
			cur = &profiles[len(profiles)-1]
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "Path="):
			cur.path = strings.TrimPrefix(line, "Path=")
		case line == "Default=1":
			cur.isDefault = true
		case line == "IsRelative=1":
			cur.isRelative = true
		}
	}

	if len(profiles) == 0 {
		return "", fmt.Errorf("no profiles found in profiles.ini")
	}

	sel := &profiles[0]
	for i := range profiles {
		if profiles[i].isDefault {
			sel = &profiles[i]
			break
		}
	}

	if sel.path == "" {
		return "", fmt.Errorf("empty profile path in profiles.ini")
	}
	if sel.isRelative {
		return filepath.Join(firefoxDir, filepath.FromSlash(sel.path)), nil
	}
	return filepath.FromSlash(sel.path), nil
}

// readClaudeAICookies copies cookies.sqlite to a temp file (to avoid Firefox's lock)
// and reads claude.ai cookies using a minimal embedded SQLite reader.
func readClaudeAICookies(dbPath string) (map[string]string, error) {
	tmp, err := os.CreateTemp("", "claude-monitor-*.sqlite")
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	src, err := os.Open(dbPath)
	if err != nil {
		tmp.Close()
		return nil, fmt.Errorf("opening %s: %w", dbPath, err)
	}
	_, copyErr := io.Copy(tmp, src)
	src.Close()
	tmp.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("copying database: %w", copyErr)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}
	return parseCookiesFromSQLite(data)
}

// ── Minimal SQLite 3 B-tree reader (read-only, no external dependencies) ────

const sqliteMagic = "SQLite format 3\x00"

type sqliteDB struct {
	data     []byte
	pageSize int
}

func newSQLiteDB(data []byte) (*sqliteDB, error) {
	if len(data) < 100 || string(data[:16]) != sqliteMagic {
		return nil, fmt.Errorf("not a valid SQLite 3 database")
	}
	ps := int(binary.BigEndian.Uint16(data[16:18]))
	if ps == 1 {
		ps = 65536
	}
	return &sqliteDB{data: data, pageSize: ps}, nil
}

func (db *sqliteDB) page(n int) []byte {
	off := (n - 1) * db.pageSize
	if off < 0 || off+db.pageSize > len(db.data) {
		return nil
	}
	return db.data[off : off+db.pageSize]
}

// readVarint reads a SQLite variable-length integer from data[pos].
// Returns (value, bytes consumed); bytes=0 on error.
func readVarint(data []byte, pos int) (int64, int) {
	var v int64
	for i := 0; i < 9 && pos+i < len(data); i++ {
		b := data[pos+i]
		if i == 8 {
			return (v << 8) | int64(b), 9
		}
		v = (v << 7) | int64(b&0x7f)
		if b&0x80 == 0 {
			return v, i + 1
		}
	}
	return 0, 0
}

// sqliteVal holds one column value from a SQLite record.
type sqliteVal struct {
	text   string
	intV   int64
	isInt  bool
	isNull bool
}

// parseRecord extracts column values from a SQLite record payload.
// Handles NULL, INTEGER (all widths), REAL, TEXT, and BLOB serial types.
func parseRecord(payload []byte) []sqliteVal {
	if len(payload) == 0 {
		return nil
	}
	hdrLen, n := readVarint(payload, 0)
	if n == 0 || int(hdrLen) < n || int(hdrLen) > len(payload) {
		return nil
	}

	var types []int64
	for pos := n; pos < int(hdrLen); {
		t, tn := readVarint(payload, pos)
		if tn == 0 {
			break
		}
		types = append(types, t)
		pos += tn
	}

	result := make([]sqliteVal, len(types))
	dataPos := int(hdrLen)

	for i, t := range types {
		v := &result[i]
		switch {
		case t == 0: // NULL
			v.isNull = true
		case t >= 1 && t <= 6: // integer of 1,2,3,4,6,8 bytes
			sizes := [7]int{0, 1, 2, 3, 4, 6, 8}
			size := sizes[t]
			if dataPos+size > len(payload) {
				return result
			}
			var iv int64
			for _, b := range payload[dataPos : dataPos+size] {
				iv = (iv << 8) | int64(b)
			}
			shift := uint(64 - size*8)
			v.intV = (iv << shift) >> shift
			v.isInt = true
			dataPos += size
		case t == 7: // 8-byte IEEE float — skip
			if dataPos+8 > len(payload) {
				return result
			}
			dataPos += 8
		case t == 8: // integer constant 0
			v.isInt = true
		case t == 9: // integer constant 1
			v.isInt = true
			v.intV = 1
		case t >= 12 && t%2 == 0: // BLOB
			size := int((t - 12) / 2)
			if dataPos+size > len(payload) {
				return result
			}
			dataPos += size
		case t >= 13 && t%2 == 1: // TEXT
			size := int((t - 13) / 2)
			if dataPos+size > len(payload) {
				return result
			}
			v.text = string(payload[dataPos : dataPos+size])
			dataPos += size
		}
	}
	return result
}

// maxInlinePayload returns the maximum bytes stored inline for this page size.
func (db *sqliteDB) maxInlinePayload() int {
	return db.pageSize - 35
}

// leafCellPayload extracts the inline record payload from a table-leaf cell.
func (db *sqliteDB) leafCellPayload(page []byte, cellOff int) []byte {
	pos := cellOff
	payloadSize, n := readVarint(page, pos)
	if n == 0 {
		return nil
	}
	pos += n
	_, n = readVarint(page, pos) // rowid — skip
	if n == 0 {
		return nil
	}
	pos += n

	inline := payloadSize
	if max := int64(db.maxInlinePayload()); inline > max {
		inline = max
	}
	end := pos + int(inline)
	if end > len(page) {
		end = len(page)
	}
	if pos >= end {
		return nil
	}
	return page[pos:end]
}

// walkTableBTree calls fn for every record in the B-tree rooted at pageNum.
// Handles both table interior (0x05) and table leaf (0x0d) pages.
func (db *sqliteDB) walkTableBTree(pageNum int, fn func([]sqliteVal)) {
	page := db.page(pageNum)
	if page == nil {
		return
	}

	hdrOff := 0
	if pageNum == 1 {
		hdrOff = 100 // page 1 carries the 100-byte database header
	}
	if len(page) < hdrOff+8 {
		return
	}

	pageType := page[hdrOff]
	cellCount := int(binary.BigEndian.Uint16(page[hdrOff+3:]))

	switch pageType {
	case 0x0d: // table leaf
		ptrStart := hdrOff + 8
		for i := 0; i < cellCount; i++ {
			pOff := ptrStart + i*2
			if pOff+2 > len(page) {
				break
			}
			cellOff := int(binary.BigEndian.Uint16(page[pOff:]))
			if cellOff >= len(page) {
				continue
			}
			if payload := db.leafCellPayload(page, cellOff); payload != nil {
				if cols := parseRecord(payload); cols != nil {
					fn(cols)
				}
			}
		}

	case 0x05: // table interior
		ptrStart := hdrOff + 12
		// Right-most child pointer is at hdrOff+8
		rightChild := int(binary.BigEndian.Uint32(page[hdrOff+8:]))
		if rightChild > 0 {
			db.walkTableBTree(rightChild, fn)
		}
		for i := 0; i < cellCount; i++ {
			pOff := ptrStart + i*2
			if pOff+2 > len(page) {
				break
			}
			cellOff := int(binary.BigEndian.Uint16(page[pOff:]))
			if cellOff+4 > len(page) {
				continue
			}
			childPage := int(binary.BigEndian.Uint32(page[cellOff:]))
			if childPage > 0 {
				db.walkTableBTree(childPage, fn)
			}
		}
	}
}

// findTableRootPage scans sqlite_master (always on page 1) for the root page
// of the given table. Returns 0 if not found.
// sqlite_master columns: type(0), name(1), tbl_name(2), rootpage(3), sql(4)
func (db *sqliteDB) findTableRootPage(tableName string) int {
	var root int
	db.walkTableBTree(1, func(cols []sqliteVal) {
		if len(cols) >= 4 &&
			cols[0].text == "table" &&
			cols[1].text == tableName &&
			cols[3].isInt {
			root = int(cols[3].intV)
		}
	})
	return root
}

// parseCookiesFromSQLite reads claude.ai cookies from raw SQLite database bytes.
// moz_cookies columns: id(0), baseDomain(1), originAttributes(2), name(3), value(4), host(5), ...
func parseCookiesFromSQLite(data []byte) (map[string]string, error) {
	db, err := newSQLiteDB(data)
	if err != nil {
		return nil, err
	}

	rootPage := db.findTableRootPage("moz_cookies")
	if rootPage == 0 {
		return nil, fmt.Errorf("moz_cookies table not found (not a Firefox cookies database?)")
	}

	cookies := make(map[string]string)
	db.walkTableBTree(rootPage, func(cols []sqliteVal) {
		if len(cols) < 6 {
			return
		}
		host := cols[5].text
		if !strings.Contains(host, "claude.ai") {
			return
		}
		name := cols[3].text
		value := cols[4].text
		if name != "" && value != "" {
			cookies[name] = value
		}
	})

	log.Printf("Found %d claude.ai cookies in Firefox profile", len(cookies))
	return cookies, nil
}
