package main

import (
	"testing"
)

// TestGenerateTokenUnique - checks that tokens are unique
func TestGenerateTokenUnique(t *testing.T) {
	token1 := generateToken()
	token2 := generateToken()

	if token1 == "" || token2 == "" {
		t.Fatal("generateToken: tokens are empty")
	}

	if token1 == token2 {
		 t.Error("generateToken: tokens should be different")
	}

	if len(token1) != 64 {
		t.Errorf("generateToken: expected length 64, got %d", len(token1))
	}
}

// TestGenerateOTPFormat - checks OTP format (6 digits)
func TestGenerateOTPFormat(t *testing.T) {
	otp := generateOTP()

	if len(otp) != 6 {
		t.Errorf("generateOTP: expected length 6, got %d (%s)", len(otp), otp)
	}

	for i, ch := range otp {
		if ch < '0' || ch > '9' {
			t.Errorf("generateOTP: char %d is not digit: %c", i, ch)
		}
	}
}

// TestGenerateOTPRandomness - checks that OTP codes are different
func TestGenerateOTPRandomness(t *testing.T) {
	codes := make(map[string]bool)
	
	for i := 0; i < 10; i++ {
		otp := generateOTP()
		if codes[otp] {
			 t.Logf("generateOTP: duplicate found at iteration %d: %s", i, otp)
		}
		codes[otp] = true
	}

	if len(codes) < 8 {
		t.Errorf("generateOTP: expected 8+ unique codes from 10 calls, got %d", len(codes))
	}
}

// TestIsValidPath - checks path validation
func TestIsValidPath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantValid bool
	}{
		{"relative path", "uploads/voice/msg.mp3", true},
		{"absolute path", "/etc/passwd", false},
		{"parent refs", "../../etc/passwd", false},
		{"simple file", "file.txt", true},
		{"nested path", "a/b/c/d.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := isValidPath(tt.path)
			if valid != tt.wantValid {
				t.Errorf("isValidPath(%q): expected %v, got %v", tt.path, tt.wantValid, valid)
			}
		})
	}
}

// TestIsValidVoicePath - checks voice file validation
func TestIsValidVoicePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantValid bool
	}{
		{"valid voice", "uploads/voice/msg.mp3", true},
		{"voice folder", "uploads/voice/", true},
		{"wrong folder", "uploads/", false},
		{"absolute path", "/uploads/voice/msg.mp3", false},
		{"no uploads prefix", "voice/msg.mp3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := isValidVoicePath(tt.path)
			if valid != tt.wantValid {
				t.Errorf("isValidVoicePath(%q): expected %v, got %v", tt.path, tt.wantValid, valid)
			}
		})
	}
}