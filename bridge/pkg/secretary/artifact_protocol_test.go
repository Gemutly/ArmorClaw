package secretary

import (
	"strings"
	"testing"
)

func TestNewArtifactEnvelope_ValidCreation(t *testing.T) {
	meta := ArtifactMetadata{
		MIMEType:  "application/pdf",
		Filename:  "report.pdf",
		SizeBytes: 1024,
	}
	env, err := NewArtifactEnvelope("@user:matrix.org", "wf-123", meta)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if env.Version != "1.0" {
		t.Fatalf("expected version 1.0, got %q", env.Version)
	}
	if env.Status != ArtifactPending {
		t.Fatalf("expected pending status, got %q", env.Status)
	}
	if env.Owner != "@user:matrix.org" {
		t.Fatalf("expected owner @user:matrix.org, got %q", env.Owner)
	}
	if env.WorkflowID != "wf-123" {
		t.Fatalf("expected workflow wf-123, got %q", env.WorkflowID)
	}
	if env.Metadata.Filename != "report.pdf" {
		t.Fatalf("expected filename report.pdf, got %q", env.Metadata.Filename)
	}
	if env.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
	if env.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero UpdatedAt")
	}
}

func TestNewArtifactEnvelope_RejectEmptyOwner(t *testing.T) {
	meta := ArtifactMetadata{Filename: "test.txt"}
	_, err := NewArtifactEnvelope("", "wf-123", meta)
	if err == nil {
		t.Fatal("expected error for empty owner")
	}
	if !strings.Contains(err.Error(), "owner") {
		t.Fatalf("expected owner error, got %q", err.Error())
	}
}

func TestNewArtifactEnvelope_RejectEmptyWorkflowID(t *testing.T) {
	meta := ArtifactMetadata{Filename: "test.txt"}
	_, err := NewArtifactEnvelope("@user:matrix.org", "", meta)
	if err == nil {
		t.Fatal("expected error for empty workflow_id")
	}
	if !strings.Contains(err.Error(), "workflow_id") {
		t.Fatalf("expected workflow_id error, got %q", err.Error())
	}
}

func TestValidate_RejectMissingID(t *testing.T) {
	env := &ArtifactEnvelope{
		Version:    "1.0",
		Status:     ArtifactPending,
		Owner:      "@user:matrix.org",
		WorkflowID: "wf-123",
	}
	err := env.Validate()
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestValidate_RejectUnsupportedVersion(t *testing.T) {
	env := &ArtifactEnvelope{
		ID:         "art-1",
		Version:    "2.0",
		Status:     ArtifactPending,
		Owner:      "@user:matrix.org",
		WorkflowID: "wf-123",
	}
	err := env.Validate()
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported version error, got %q", err.Error())
	}
}

func TestValidate_RejectInvalidStatus(t *testing.T) {
	env := &ArtifactEnvelope{
		ID:         "art-1",
		Version:    "1.0",
		Status:     ArtifactStatus("unknown"),
		Owner:      "@user:matrix.org",
		WorkflowID: "wf-123",
	}
	err := env.Validate()
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestValidate_RejectPathTraversalFilename(t *testing.T) {
	meta := ArtifactMetadata{Filename: "../../../etc/passwd"}
	env := &ArtifactEnvelope{
		ID:         "art-1",
		Version:    "1.0",
		Status:     ArtifactPending,
		Metadata:   meta,
		Owner:      "@user:matrix.org",
		WorkflowID: "wf-123",
	}
	err := env.Validate()
	if err == nil {
		t.Fatal("expected error for path traversal in filename")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("expected path traversal error, got %q", err.Error())
	}
}

func TestValidate_RejectPathSeparatorFilename(t *testing.T) {
	for _, fn := range []string{"dir/file.txt", "dir\\file.txt"} {
		meta := ArtifactMetadata{Filename: fn}
		env := &ArtifactEnvelope{
			ID:         "art-1",
			Version:    "1.0",
			Status:     ArtifactPending,
			Metadata:   meta,
			Owner:      "@user:matrix.org",
			WorkflowID: "wf-123",
		}
		err := env.Validate()
		if err == nil {
			t.Fatalf("expected error for filename %q", fn)
		}
		if !strings.Contains(err.Error(), "path separator") {
			t.Fatalf("expected path separator error for %q, got %q", fn, err.Error())
		}
	}
}

func TestValidate_RejectInvalidMIMEType(t *testing.T) {
	meta := ArtifactMetadata{MIMEType: "not-a-mime-type"}
	env := &ArtifactEnvelope{
		ID:         "art-1",
		Version:    "1.0",
		Status:     ArtifactPending,
		Metadata:   meta,
		Owner:      "@user:matrix.org",
		WorkflowID: "wf-123",
	}
	err := env.Validate()
	if err == nil {
		t.Fatal("expected error for invalid MIME type")
	}
}

func TestValidate_AcceptValidMIMETypes(t *testing.T) {
	for _, mt := range []string{
		"application/pdf",
		"text/plain",
		"image/png",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	} {
		meta := ArtifactMetadata{MIMEType: mt}
		env := &ArtifactEnvelope{
			ID:         "art-1",
			Version:    "1.0",
			Status:     ArtifactPending,
			Metadata:   meta,
			Owner:      "@user:matrix.org",
			WorkflowID: "wf-123",
		}
		if err := env.Validate(); err != nil {
			t.Fatalf("valid MIME type %q rejected: %v", mt, err)
		}
	}
}

func TestValidate_RejectEmptyTag(t *testing.T) {
	meta := ArtifactMetadata{Tags: []string{"valid", "  ", ""}}
	env := &ArtifactEnvelope{
		ID:         "art-1",
		Version:    "1.0",
		Status:     ArtifactPending,
		Metadata:   meta,
		Owner:      "@user:matrix.org",
		WorkflowID: "wf-123",
	}
	err := env.Validate()
	if err == nil {
		t.Fatal("expected error for empty tag")
	}
}

func TestValidate_RejectBadChecksumFormat(t *testing.T) {
	env := &ArtifactEnvelope{
		ID:         "art-1",
		Version:    "1.0",
		Status:     ArtifactPending,
		Owner:      "@user:matrix.org",
		WorkflowID: "wf-123",
		Checksum:   "not-a-sha256",
	}
	err := env.Validate()
	if err == nil {
		t.Fatal("expected error for bad checksum format")
	}
}

func TestValidate_AcceptValidChecksum(t *testing.T) {
	env := &ArtifactEnvelope{
		ID:         "art-1",
		Version:    "1.0",
		Status:     ArtifactPending,
		Owner:      "@user:matrix.org",
		WorkflowID: "wf-123",
		Checksum:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}
	if err := env.Validate(); err != nil {
		t.Fatalf("valid checksum rejected: %v", err)
	}
}

func TestSupportsVersion(t *testing.T) {
	if !SupportsVersion("1.0") {
		t.Fatal("expected version 1.0 to be supported")
	}
	if SupportsVersion("2.0") {
		t.Fatal("expected version 2.0 to NOT be supported")
	}
}

func TestTransitionStatus_ValidTransitions(t *testing.T) {
	env, _ := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})

	if err := env.TransitionStatus(ArtifactProcessing); err != nil {
		t.Fatalf("pending->processing: %v", err)
	}
	if err := env.TransitionStatus(ArtifactCompleted); err != nil {
		t.Fatalf("processing->completed: %v", err)
	}
}

func TestTransitionStatus_InvalidTransition(t *testing.T) {
	env, _ := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})

	err := env.TransitionStatus(ArtifactCompleted)
	if err == nil {
		t.Fatal("expected error: cannot go pending->completed directly")
	}
}

func TestTransitionStatus_ExpiredTerminal(t *testing.T) {
	env, _ := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})
	env.Status = ArtifactExpired

	err := env.TransitionStatus(ArtifactPending)
	if err == nil {
		t.Fatal("expected error: expired is terminal")
	}
}

func TestTransitionStatus_FailedRetry(t *testing.T) {
	env, _ := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})
	env.Status = ArtifactFailed

	if err := env.TransitionStatus(ArtifactPending); err != nil {
		t.Fatalf("failed->pending retry should work: %v", err)
	}
}

func TestSetChecksum(t *testing.T) {
	env, _ := NewArtifactEnvelope("@u:m", "wf-1", ArtifactMetadata{})
	content := []byte("hello world")
	env.SetChecksum(content)

	// SHA-256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if env.Checksum != expected {
		t.Fatalf("expected checksum %q, got %q", expected, env.Checksum)
	}
}

func TestSanitizeMetadata(t *testing.T) {
	raw := ArtifactMetadata{
		MIMEType:    "  application/pdf  ",
		Filename:    "  report.pdf  ",
		Source:      "  email  ",
		Destination: "  mobile  ",
		Tags:        []string{"  finance  ", "  ", "", "  q4  "},
	}
	clean := SanitizeMetadata(raw)

	if clean.MIMEType != "application/pdf" {
		t.Fatalf("expected trimmed MIMEType, got %q", clean.MIMEType)
	}
	if clean.Filename != "report.pdf" {
		t.Fatalf("expected trimmed Filename, got %q", clean.Filename)
	}
	if len(clean.Tags) != 2 {
		t.Fatalf("expected 2 clean tags, got %d: %v", len(clean.Tags), clean.Tags)
	}
	if clean.Tags[0] != "finance" || clean.Tags[1] != "q4" {
		t.Fatalf("expected [finance, q4], got %v", clean.Tags)
	}
}

func TestValidArtifactStatuses(t *testing.T) {
	expected := []ArtifactStatus{
		ArtifactPending, ArtifactProcessing, ArtifactCompleted,
		ArtifactFailed, ArtifactExpired,
	}
	for _, s := range expected {
		if !ValidArtifactStatuses[s] {
			t.Fatalf("expected %q to be valid", s)
		}
	}
}
