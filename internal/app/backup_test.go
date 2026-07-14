package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
)

func TestAutomatedBackupPublishesDecryptableValidatedSnapshot(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	srv := newTestServerWithConfig(t, Config{Addr: "127.0.0.1:0", DatabasePath: filepath.Join(t.TempDir(), "db.sqlite"), OperatorToken: "operator-secret", JWTSecret: "secret", STUNServers: []string{"stun:test"}, BackupDirectory: dir, BackupAgeRecipient: identity.Recipient().String(), BackupTimezone: "UTC"})
	deadline := time.Now().Add(5 * time.Second)
	for srv.backup.snapshotStatus().Status != "ok" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	status := srv.backup.snapshotStatus()
	if status.Status != "ok" {
		t.Fatalf("status=%+v", status)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.sqlite.age"))
	if len(files) != 1 {
		t.Fatalf("files=%v", files)
	}
	if _, err = os.Stat(files[0] + ".json"); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(files[0])
	if err != nil {
		t.Fatal(err)
	}
	reader, err := age.Decrypt(in, identity)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plain), "SQLite format 3") {
		t.Fatal("decrypted backup is not SQLite")
	}
	identityPath := filepath.Join(t.TempDir(), "identity.txt")
	if err = os.WriteFile(identityPath, []byte(identity.String()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(t.TempDir(), "restored.db")
	if err = RestoreBackup(files[0], restored, identityPath); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(restored); err != nil {
		t.Fatal(err)
	}
}

func TestBackupRequiresAgeRecipient(t *testing.T) {
	_, err := NewServer(Config{Addr: "127.0.0.1:0", DatabasePath: filepath.Join(t.TempDir(), "db.sqlite"), OperatorToken: "operator-secret", JWTSecret: "secret", STUNServers: []string{"stun:test"}, BackupDirectory: t.TempDir(), BackupTimezone: "UTC"})
	if err == nil || !strings.Contains(err.Error(), "recipient") {
		t.Fatalf("err=%v", err)
	}
}
