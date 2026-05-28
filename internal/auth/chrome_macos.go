//go:build darwin

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/pbkdf2"
)

// chromeCookieDB returns the path to Chrome's Cookies SQLite file for the
// default profile on macOS.
func chromeCookieDB() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// Try Chrome first, then Chrome Canary, then Brave (Chromium derivative).
	candidates := []string{
		filepath.Join(home, "Library/Application Support/Google/Chrome/Default/Cookies"),
		filepath.Join(home, "Library/Application Support/Google/Chrome Canary/Default/Cookies"),
		filepath.Join(home, "Library/Application Support/BraveSoftware/Brave-Browser/Default/Cookies"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no supported Chromium cookie DB found (Chrome/Canary/Brave)")
}

// chromeSafeStoragePassword reads the "Chrome Safe Storage" password from the
// macOS keychain. This is the master key used to derive the AES key that
// encrypts each cookie value.
//
// Prompts the user via the system keychain dialog on first call per session.
// account selects the browser ("Chrome", "Chromium", "Brave", etc.).
func chromeSafeStoragePassword(account string) ([]byte, error) {
	if account == "" {
		account = "Chrome"
	}
	service := account + " Safe Storage"
	// Service-scoped lookup is precise (there may be multiple keychain entries
	// with account "Chrome").
	out, err := exec.Command("security", "find-generic-password", "-s", service, "-a", account, "-w").Output()
	if err == nil {
		return []byte(strings.TrimSpace(string(out))), nil
	}
	// Fallback to less-specific account-only lookup for older Chrome versions.
	out, err2 := exec.Command("security", "find-generic-password", "-a", account, "-w").Output()
	if err2 != nil {
		return nil, fmt.Errorf("read %q from Keychain: %w (you may need to approve the keychain dialog)", service, err)
	}
	return []byte(strings.TrimSpace(string(out))), nil
}

// deriveChromeAESKey applies Chrome's PBKDF2(SHA1, salt="saltysalt", iter=1003, len=16)
// to the safe-storage password to produce the AES-128-CBC key used for "v10" cookies.
func deriveChromeAESKey(password []byte) []byte {
	return pbkdf2.Key(password, []byte("saltysalt"), 1003, 16, sha1.New)
}

// decryptChromeCookieV10 decrypts a v10 cookie. v10 cookies start with the
// literal bytes "v10", followed by AES-128-CBC ciphertext with IV = 16 spaces.
func decryptChromeCookieV10(enc, key []byte) ([]byte, error) {
	if len(enc) < 3 || string(enc[:3]) != "v10" {
		return nil, errors.New("not a v10 cookie")
	}
	ct := enc[3:]
	if len(ct)%aes.BlockSize != 0 || len(ct) == 0 {
		return nil, fmt.Errorf("v10 ciphertext length %d invalid", len(ct))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := bytes16Spaces()
	mode := cipher.NewCBCDecrypter(block, iv)
	pt := make([]byte, len(ct))
	mode.CryptBlocks(pt, ct)
	return pkcs7Unpad(pt)
}

func bytes16Spaces() []byte {
	b := make([]byte, 16)
	for i := range b {
		b[i] = ' '
	}
	return b
}

func pkcs7Unpad(b []byte) ([]byte, error) {
	if len(b) == 0 {
		return nil, errors.New("empty plaintext")
	}
	n := int(b[len(b)-1])
	if n == 0 || n > len(b) || n > aes.BlockSize {
		return nil, fmt.Errorf("invalid pkcs7 pad: %d", n)
	}
	return b[:len(b)-n], nil
}

// openChromeDB copies Chrome's cookie DB (plus its WAL/SHM siblings if
// present) to a temp dir and opens it read-only. Copying the WAL is what lets
// us read while Chrome is open — without it the main file lags behind.
func openChromeDB() (*sql.DB, func(), error) {
	dbPath, err := chromeCookieDB()
	if err != nil {
		return nil, nil, err
	}
	tmpDir, err := os.MkdirTemp("", "slk-cookies-*")
	if err != nil {
		return nil, nil, err
	}
	tmp := filepath.Join(tmpDir, "Cookies")
	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	if err := copyFileTo(dbPath, tmp); err != nil {
		cleanup()
		return nil, nil, err
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		src := dbPath + suffix
		if _, err := os.Stat(src); err == nil {
			_ = copyFileTo(src, tmp+suffix)
		}
	}
	db, err := sql.Open("sqlite3", tmp+"?_busy_timeout=2000&mode=ro&immutable=1")
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return db, func() { _ = db.Close(); cleanup() }, nil
}

// ExtractXOXDFromChrome reads Chrome's cookie DB and decrypts the "d" cookie
// on *.slack.com. Tries with Chrome open by copying DB + WAL together.
//
// Slack has multiple cookies named "d" across hosts (apex `.slack.com` is the
// xoxd session; per-workspace `myco.slack.com` may have its own metadata `d`).
// We try each, decrypt, and return the first that looks like an xoxd token.
func ExtractXOXDFromChrome() (string, error) {
	db, cleanup, err := openChromeDB()
	if err != nil {
		return "", err
	}
	defer cleanup()

	rows, err := db.Query(`
		SELECT host_key, encrypted_value FROM cookies
		WHERE name = 'd'
		  AND (host_key = '.slack.com' OR host_key LIKE '%.slack.com')
		ORDER BY (host_key = '.slack.com') DESC, length(encrypted_value) ASC`)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	pw, err := chromeSafeStoragePassword("Chrome")
	if err != nil {
		return "", err
	}
	key := deriveChromeAESKey(pw)

	var candidates int
	var lastErr error
	for rows.Next() {
		var host string
		var enc []byte
		if err := rows.Scan(&host, &enc); err != nil {
			lastErr = err
			continue
		}
		candidates++
		pt, err := decryptChromeCookie(enc, key)
		if err != nil {
			lastErr = err
			continue
		}
		s := strings.TrimSpace(string(pt))
		if strings.HasPrefix(s, "xoxd-") {
			return s, nil
		}
	}
	if candidates == 0 {
		return "", errors.New("no 'd' cookie for *.slack.com in Chrome DB (sign into Slack in Chrome first)")
	}
	if lastErr != nil {
		return "", fmt.Errorf("decrypt 'd' cookie failed (%d candidates tried, likely v20 App-Bound encryption — paste manually): %w", candidates, lastErr)
	}
	return "", fmt.Errorf("found %d 'd' cookie(s) but none decrypted to xoxd-* (likely v20 App-Bound encryption; paste manually)", candidates)
}

// decryptChromeCookie dispatches based on the version prefix. Returns a
// helpful error for v20 since that scheme is App-Bound and requires more than
// the standard "Chrome Safe Storage" Keychain key on macOS.
func decryptChromeCookie(enc, key []byte) ([]byte, error) {
	if len(enc) < 3 {
		return nil, errors.New("encrypted value too short")
	}
	prefix := string(enc[:3])
	switch prefix {
	case "v10":
		return decryptChromeCookieV10(enc, key)
	case "v20":
		return nil, errors.New("v20 App-Bound cookie (Chrome 127+) — not yet supported on macOS")
	}
	return nil, fmt.Errorf("unknown cookie version prefix %q", prefix)
}

// ScanSlackTeamsFromChrome returns the set of workspace subdomains the user
// has visited (one entry per distinct <team>.slack.com host_key in cookies).
// Reserved hostnames (app/api/files/edgeapi/etc.) are filtered out.
func ScanSlackTeamsFromChrome() ([]string, error) {
	db, cleanup, err := openChromeDB()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	rows, err := db.Query(`SELECT DISTINCT host_key FROM cookies WHERE host_key LIKE '%.slack.com'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			continue
		}
		h = strings.TrimPrefix(h, ".")
		if !strings.HasSuffix(h, ".slack.com") {
			continue
		}
		sub := strings.TrimSuffix(h, ".slack.com")
		if strings.Contains(sub, ".") {
			continue // multi-level (e.g. edge-cache.slack.com)
		}
		switch sub {
		case "app", "api", "www", "files", "edgeapi", "downloads",
			"slack-files", "slack-edge", "slack-imgs", "a", "ca", "wss-primary",
			"wss-backup", "wss-mobile", "status":
			continue
		}
		seen[sub] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

func copyFileTo(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
