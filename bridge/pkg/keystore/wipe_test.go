// Package keystore_test provides tests for the keystore.WipeAllData() function
// and flow-level wipe verification (Plan Task 11, Phase S6).
//
// WipeAllData removes ALL data from the keystore.
// This is a destructive operation that should only be called during readmin process.
// Security audit: This test verifies that WipeAllData respects user isolation.

//go:build cgo

package keystore_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/armorclaw/bridge/pkg/keystore"
)

// keystorePkg returns the filesystem path to the keystore package source directory.
func keystorePkg(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(thisFile)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", path, err)
	}
	return string(data)
}

func extractFuncBody(t *testing.T, src, funcName string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source for %s: %v", funcName, err)
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == funcName {
			var buf strings.Builder
			for _, stmt := range fn.Body.List {
				start := fset.Position(stmt.Pos())
				end := fset.Position(stmt.End())
				lines := strings.Split(src, "\n")
				for i := start.Line - 1; i < end.Line && i < len(lines); i++ {
					buf.WriteString(lines[i])
					buf.WriteString("\n")
				}
			}
			return buf.String()
		}
	}
	t.Fatalf("Function %s not found in source", funcName)
	return ""
}

// ---------------------------------------------------------------------------
// Existing WipeAllData tests
// ---------------------------------------------------------------------------

func TestWipeAllData_SystemWide(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "keystore.db")

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	ks, err := keystore.New(keystore.Config{
		DBPath:    dbPath,
		MasterKey: masterKey,
	})
	if err != nil {
		t.Fatalf("Failed to create keystore: %v", err)
	}

	if err := ks.Open(); err != nil {
		t.Fatalf("Failed to open keystore: %v", err)
	}
	defer ks.Close()

	err = ks.Store(keystore.Credential{
		Provider: "openai",
		Token:    "test-token-1",
	})
	if err != nil {
		t.Fatalf("Failed to store credential: %v", err)
	}

	creds, err := ks.List("")
	if err != nil {
		t.Fatalf("Failed to list credentials: %v", err)
	}
	if len(creds) == 0 {
		t.Fatal("Expected at least one credential before wipe")
	}

	err = ks.WipeAllData()
	if err != nil {
		t.Fatalf("WipeAllData failed: %v", err)
	}

	creds, err = ks.List("")
	if err != nil {
		t.Fatalf("Failed to list credentials after wipe: %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("Expected 0 credentials after WipeAllData, got %d", len(creds))
	}

	t.Log("WipeAllData correctly performs system-wide deletion")
}

func TestWipeAllData_RequiresOpenKeystore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "keystore.db")

	masterKey := make([]byte, 32)
	for i := range masterKey {
		masterKey[i] = byte(i)
	}

	ks, err := keystore.New(keystore.Config{
		DBPath:    dbPath,
		MasterKey: masterKey,
	})
	if err != nil {
		t.Fatalf("Failed to create keystore: %v", err)
	}

	err = ks.WipeAllData()
	if err == nil {
		t.Error("Expected WipeAllData to fail on unopened keystore")
	}

	t.Logf("WipeAllData correctly rejected: %v", err)
}

// ---------------------------------------------------------------------------
// Flow-Level Wipe Verification Tests (Plan Task 11, Phase S6)
// ---------------------------------------------------------------------------
//
// Strategy:
//   PRIMARY  — Code-audit verification: grep source for ZeroBytes() calls at
//              every secret lifecycle endpoint.
//   SECONDARY — Runtime verification where byte slices remain in scope.
//   NOT DONE — Verify Go GC zeroed all copies (impossible by design).
// ---------------------------------------------------------------------------

func TestWipeAudit_DerivedKeyInWrapKey(t *testing.T) {
	src := readFile(t, filepath.Join(keystorePkg(t), "key_derivation.go"))
	if !strings.Contains(src, "func (kd *KeyDerivation) WrapKey(") {
		t.Fatal("WrapKey function not found in key_derivation.go")
	}
	wrapKeyBody := extractFuncBody(t, src, "WrapKey")
	if !strings.Contains(wrapKeyBody, "ZeroBytes(derived.Key)") {
		t.Error("WrapKey is MISSING ZeroBytes(derived.Key) — derived KEK not wiped")
	} else {
		t.Log("PASS: WrapKey wipes derived.Key via defer ZeroBytes")
	}
}

func TestWipeAudit_DerivedKeyInUnwrapKey(t *testing.T) {
	src := readFile(t, filepath.Join(keystorePkg(t), "key_derivation.go"))
	if !strings.Contains(src, "func (kd *KeyDerivation) UnwrapKey(") {
		t.Fatal("UnwrapKey function not found in key_derivation.go")
	}
	body := extractFuncBody(t, src, "UnwrapKey")
	if !strings.Contains(body, "ZeroBytes(derived.Key)") {
		t.Error("UnwrapKey is MISSING ZeroBytes(derived.Key) — derived KEK not wiped")
	} else {
		t.Log("PASS: UnwrapKey wipes derived.Key via defer ZeroBytes")
	}
}

func TestWipeAudit_PlaintextInRekey(t *testing.T) {
	src := readFile(t, filepath.Join(keystorePkg(t), "key_derivation.go"))
	body := extractFuncBody(t, src, "Rekey")
	if !strings.Contains(body, "ZeroBytes(plaintext)") {
		t.Error("Rekey is MISSING ZeroBytes(plaintext) — decrypted key not wiped")
	} else {
		t.Log("PASS: Rekey wipes plaintext via defer ZeroBytes")
	}
}

func TestWipeAudit_PlaintextInChangeParams(t *testing.T) {
	src := readFile(t, filepath.Join(keystorePkg(t), "key_derivation.go"))
	body := extractFuncBody(t, src, "ChangeParams")
	if !strings.Contains(body, "ZeroBytes(plaintext)") {
		t.Error("ChangeParams is MISSING ZeroBytes(plaintext) — decrypted key not wiped")
	} else {
		t.Log("PASS: ChangeParams wipes plaintext via defer ZeroBytes")
	}
}

func TestWipeAudit_VaultKeyOnSeal(t *testing.T) {
	src := readFile(t, filepath.Join(keystorePkg(t), "sealed_keystore.go"))

	sealBody := extractFuncBody(t, src, "Seal")
	if !strings.Contains(sealBody, "ZeroBytes(sk.vaultKey)") {
		t.Error("Seal is MISSING ZeroBytes(sk.vaultKey) — vault key may persist in memory")
	} else {
		t.Log("PASS: Seal wipes sk.vaultKey")
	}

	sealPwdBody := extractFuncBody(t, src, "SealPassword")
	if !strings.Contains(sealPwdBody, "ZeroBytes(sk.vaultKey)") {
		t.Error("SealPassword is MISSING ZeroBytes(sk.vaultKey) — vault key may persist in memory")
	} else {
		t.Log("PASS: SealPassword wipes sk.vaultKey")
	}
}

func TestWipeAudit_PasswordVerifierInUnseal(t *testing.T) {
	src := readFile(t, filepath.Join(keystorePkg(t), "sealed_keystore.go"))
	body := extractFuncBody(t, src, "UnsealWithPassword")

	count := strings.Count(body, "ZeroBytes(candidateBytes)")
	if count < 2 {
		t.Errorf("UnsealWithPassword only has %d ZeroBytes(candidateBytes) call(s), expected ≥ 2 (success + failure paths)", count)
	} else {
		t.Logf("PASS: UnsealWithPassword wipes candidateBytes on %d paths", count)
	}
}

func TestWipeAudit_ZeroBytesFunctionExists(t *testing.T) {
	src := readFile(t, filepath.Join(keystorePkg(t), "securemem.go"))
	if !strings.Contains(src, "func ZeroBytes(b []byte)") {
		t.Fatal("ZeroBytes function not found in securemem.go")
	}
	if !strings.Contains(src, "b[i] = 0") {
		t.Error("ZeroBytes does not perform byte-by-byte zeroing")
	}
	if !strings.Contains(src, "runtime.KeepAlive(b)") {
		t.Error("ZeroBytes is MISSING runtime.KeepAlive — compiler may optimize away the zeroing")
	} else {
		t.Log("PASS: ZeroBytes uses byte-by-byte zeroing + runtime.KeepAlive")
	}
}

func TestWipeAudit_AllSecretPathsCovered(t *testing.T) {
	pkgDir := keystorePkg(t)

	criticalFiles := []string{
		"key_derivation.go",
		"sealed_keystore.go",
		"securemem.go",
	}

	for _, name := range criticalFiles {
		path := filepath.Join(pkgDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Critical file %s not found", name)
			continue
		}
		t.Logf("Verified %s exists", name)
	}

	kdSrc := readFile(t, filepath.Join(pkgDir, "key_derivation.go"))
	skSrc := readFile(t, filepath.Join(pkgDir, "sealed_keystore.go"))

	totalKD := strings.Count(kdSrc, "ZeroBytes(")
	totalSK := strings.Count(skSrc, "ZeroBytes(")

	t.Logf("ZeroBytes calls in key_derivation.go: %d", totalKD)
	t.Logf("ZeroBytes calls in sealed_keystore.go: %d", totalSK)

	if totalKD < 4 {
		t.Errorf("key_derivation.go has only %d ZeroBytes calls, expected ≥ 4", totalKD)
	}
	if totalSK < 4 {
		t.Errorf("sealed_keystore.go has only %d ZeroBytes calls, expected ≥ 4", totalSK)
	}
}

func TestWipeAudit_NoLogImportsInKeystore(t *testing.T) {
	pkgDir := keystorePkg(t)
	criticalFiles := []string{"key_derivation.go", "sealed_keystore.go", "securemem.go"}

	forbiddenImports := []string{
		"\"log\"",
		"log/",
	}

	for _, name := range criticalFiles {
		src := readFile(t, filepath.Join(pkgDir, name))
		for _, forbidden := range forbiddenImports {
			if strings.Contains(src, forbidden) {
				t.Errorf("%s imports %q — potential secret material leak via logging", name, forbidden)
			}
		}
	}
	t.Log("PASS: No log imports in critical keystore files")
}

func TestWipeAudit_NoSecretMaterialInLogCalls(t *testing.T) {
	pkgDir := keystorePkg(t)
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("Failed to read keystore directory: %v", err)
	}

	dangerousPatterns := []string{
		"log.Print",
		"log.Printf",
		"log.Println",
		"fmt.Print",
		"fmt.Printf",
		"fmt.Println",
	}

	secretVars := []string{
		"password",
		"passwd",
		"token",
		"secret",
		"vaultKey",
		"candidateBytes",
		"plaintext",
		"masterKey",
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}

		src := readFile(t, filepath.Join(pkgDir, entry.Name()))
		lines := strings.Split(src, "\n")

		for i, line := range lines {
			for _, pattern := range dangerousPatterns {
				if strings.Contains(line, pattern) {
					lineLower := strings.ToLower(line)
					for _, sv := range secretVars {
						if strings.Contains(lineLower, strings.ToLower(sv)) {
							t.Errorf("%s:%d — potential secret leak: %s references %q",
								entry.Name(), i+1, pattern, sv)
						}
					}
				}
			}
		}
	}
	t.Log("PASS: No secret material found in print/log calls")
}
