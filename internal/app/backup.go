package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
)

type backupStatus struct {
	Status      string `json:"status"`
	LastSuccess string `json:"last_success,omitempty"`
	NextAttempt string `json:"next_attempt,omitempty"`
	Error       string `json:"error,omitempty"`
}
type backupManager struct {
	server    *Server
	recipient age.Recipient
	location  *time.Location
	stop      chan struct{}
	done      chan struct{}
	mu        sync.RWMutex
	status    backupStatus
	runMu     sync.Mutex
}

func newBackupManager(server *Server) (*backupManager, error) {
	if server.cfg.BackupAgeRecipient == "" {
		return nil, fmt.Errorf("backup age recipient is required")
	}
	recipient, err := age.ParseX25519Recipient(server.cfg.BackupAgeRecipient)
	if err != nil {
		return nil, fmt.Errorf("parse backup age recipient: %w", err)
	}
	location, err := time.LoadLocation(server.cfg.BackupTimezone)
	if err != nil {
		return nil, fmt.Errorf("load backup timezone: %w", err)
	}
	if err = os.MkdirAll(server.cfg.BackupDirectory, 0700); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}
	return &backupManager{server: server, recipient: recipient, location: location, stop: make(chan struct{}), done: make(chan struct{}), status: backupStatus{Status: "stale"}}, nil
}

func (m *backupManager) start() {
	go func() {
		defer close(m.done)
		_ = m.run(context.Background())
		for {
			next := nextBackupAt(time.Now(), m.location)
			m.setNext(next)
			timer := time.NewTimer(time.Until(next))
			select {
			case <-timer.C:
				_ = m.run(context.Background())
			case <-m.stop:
				timer.Stop()
				return
			}
		}
	}()
}
func (m *backupManager) close() { close(m.stop); <-m.done }
func nextBackupAt(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	next := time.Date(local.Year(), local.Month(), local.Day(), 3, 0, 0, 0, location)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
func (m *backupManager) setNext(next time.Time) {
	m.mu.Lock()
	m.status.NextAttempt = next.UTC().Format(time.RFC3339)
	m.mu.Unlock()
}
func (m *backupManager) snapshotStatus() backupStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}
func (m *backupManager) fail(err error) error {
	m.mu.Lock()
	m.status.Status = "failed"
	m.status.Error = err.Error()
	m.mu.Unlock()
	return err
}

func (m *backupManager) run(ctx context.Context) error {
	m.runMu.Lock()
	defer m.runMu.Unlock()
	stamp := time.Now().UTC().Format("20060102T150405Z")
	snapshot := filepath.Join(m.server.cfg.BackupDirectory, "."+stamp+".sqlite.tmp")
	encryptedTmp := filepath.Join(m.server.cfg.BackupDirectory, "."+stamp+".age.tmp")
	final := filepath.Join(m.server.cfg.BackupDirectory, "private-direct-"+stamp+".sqlite.age")
	defer os.Remove(snapshot)
	defer os.Remove(encryptedTmp)
	if _, err := m.server.db.ExecContext(ctx, "VACUUM INTO ?", snapshot); err != nil {
		return m.fail(fmt.Errorf("snapshot database: %w", err))
	}
	check, err := sql.Open("sqlite", snapshot)
	if err != nil {
		return m.fail(err)
	}
	var integrity string
	err = check.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity)
	check.Close()
	if err != nil || integrity != "ok" {
		return m.fail(fmt.Errorf("backup integrity check failed"))
	}
	in, err := os.Open(snapshot)
	if err != nil {
		return m.fail(err)
	}
	out, err := os.OpenFile(encryptedTmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		in.Close()
		return m.fail(err)
	}
	writer, err := age.Encrypt(out, m.recipient)
	if err != nil {
		in.Close()
		out.Close()
		return m.fail(fmt.Errorf("encrypt backup: %w", err))
	}
	_, err = io.Copy(writer, in)
	if closeErr := writer.Close(); err == nil {
		err = closeErr
	}
	in.Close()
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return m.fail(fmt.Errorf("encrypt backup: %w", err))
	}
	if err = os.Rename(encryptedTmp, final); err != nil {
		return m.fail(err)
	}
	data, err := os.ReadFile(final)
	if err != nil {
		return m.fail(err)
	}
	sum := sha256.Sum256(data)
	var version int
	_ = m.server.db.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version)
	manifest := map[string]any{"file": filepath.Base(final), "sha256": hex.EncodeToString(sum[:]), "size": len(data), "created_at": time.Now().UTC().Format(time.RFC3339), "schema_version": version}
	manifestData, _ := json.Marshal(manifest)
	if err = os.WriteFile(final+".json", manifestData, 0600); err != nil {
		return m.fail(err)
	}
	m.prune()
	m.mu.Lock()
	m.status.Status = "ok"
	m.status.Error = ""
	m.status.LastSuccess = time.Now().UTC().Format(time.RFC3339)
	m.mu.Unlock()
	return nil
}

func (m *backupManager) prune() {
	entries, _ := filepath.Glob(filepath.Join(m.server.cfg.BackupDirectory, "private-direct-*.sqlite.age"))
	sort.Sort(sort.Reverse(sort.StringSlice(entries)))
	keep := map[string]bool{}
	weeks := map[string]bool{}
	for i, path := range entries {
		base := filepath.Base(path)
		stamp := strings.TrimSuffix(strings.TrimPrefix(base, "private-direct-"), ".sqlite.age")
		parsed, err := time.Parse("20060102T150405Z", stamp)
		if err != nil {
			continue
		}
		if i < 7 {
			keep[path] = true
		}
		year, week := parsed.ISOWeek()
		key := fmt.Sprintf("%d-%d", year, week)
		if len(weeks) < 4 && !weeks[key] {
			weeks[key] = true
			keep[path] = true
		}
	}
	for _, path := range entries {
		if !keep[path] {
			_ = os.Remove(path)
			_ = os.Remove(path + ".json")
		}
	}
}

func (s *Server) handleBackupHealth(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Operator-Token") != s.cfg.OperatorToken {
		writeError(w, 401, "unauthorized")
		return
	}
	if s.backup == nil {
		writeJSON(w, 200, backupStatus{Status: "disabled"})
		return
	}
	writeJSON(w, 200, s.backup.snapshotStatus())
}

func RestoreBackup(encryptedPath, outputPath, identityPath string) error {
	identityFile, err := os.Open(identityPath)
	if err != nil {
		return err
	}
	identities, err := age.ParseIdentities(identityFile)
	identityFile.Close()
	if err != nil {
		return err
	}
	input, err := os.Open(encryptedPath)
	if err != nil {
		return err
	}
	decrypted, err := age.Decrypt(input, identities...)
	if err != nil {
		input.Close()
		return err
	}
	temporary := outputPath + ".tmp"
	out, err := os.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		input.Close()
		return err
	}
	_, err = io.Copy(out, decrypted)
	input.Close()
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(temporary)
		return err
	}
	check, err := sql.Open("sqlite", temporary)
	if err != nil {
		os.Remove(temporary)
		return err
	}
	var integrity string
	err = check.QueryRow("PRAGMA integrity_check").Scan(&integrity)
	check.Close()
	if err != nil || integrity != "ok" {
		os.Remove(temporary)
		return fmt.Errorf("restored database integrity check failed")
	}
	return os.Rename(temporary, outputPath)
}
