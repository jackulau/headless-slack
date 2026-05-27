//go:build darwin

package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"crypto/sha1"
	"golang.org/x/crypto/pbkdf2"

	_ "github.com/mattn/go-sqlite3"
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
func chromeSafeStoragePassword(service string) ([]byte, error) {
	if service == "" {
		service = "Chrome"
	}
	out, err := exec.Command("security", "find-generic-password", "-wa", service).Output()
	if err != nil {
		return nil, fmt.Errorf("read %q safe storage password: %w (you may need to approve the keychain dialog)", service, err)
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

// ExtractXOXDFromChrome reads Chrome's cookie DB and decrypts the "d" cookie
// on *.slack.com. Chrome must be closed (it holds an exclusive lock).
func ExtractXOXDFromChrome() (string, error) {
	dbPath, err := chromeCookieDB()
	if err != nil {
		return "", err
	}
	// Copy DB to a temp file so we don't fight Chrome for the lock if it's
	// running with the WAL still active.
	tmp, err := copyFile(dbPath)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp)

	db, err := sql.Open("sqlite3", tmp+"?_busy_timeout=2000&mode=ro")
	if err != nil {
		return "", err
	}
	defer db.Close()

	row := db.QueryRow(`
		SELECT encrypted_value FROM cookies
		WHERE name = 'd'
		  AND host_key LIKE '%.slack.com'
		ORDER BY length(encrypted_value) DESC
		LIMIT 1`)
	var enc []byte
	if err := row.Scan(&enc); err != nil {
		return "", fmt.Errorf("no 'd' cookie for *.slack.com in Chrome DB (sign into Slack in Chrome first): %w", err)
	}

	pw, err := chromeSafeStoragePassword("Chrome")
	if err != nil {
		return "", err
	}
	key := deriveChromeAESKey(pw)
	pt, err := decryptChromeCookieV10(enc, key)
	if err != nil {
		return "", fmt.Errorf("decrypt 'd' cookie (Chrome v20 not yet supported; downgrade or paste manually): %w", err)
	}
	s := strings.TrimSpace(string(pt))
	if !strings.HasPrefix(s, "xoxd-") {
		return "", fmt.Errorf("decrypted cookie does not look like xoxd (len=%d)", len(s))
	}
	return s, nil
}

func copyFile(src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	tmp, err := os.CreateTemp("", "slk-cookies-*.sqlite")
	if err != nil {
		return "", err
	}
	defer tmp.Close()
	buf := make([]byte, 1<<16)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				return "", werr
			}
		}
		if err != nil {
			break
		}
	}
	return tmp.Name(), nil
}
