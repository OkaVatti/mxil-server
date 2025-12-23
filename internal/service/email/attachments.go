package email

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (s *emailService) validateAttachment(att AttachmentInfo) error {
	if att.Filename == "" {
		return fmt.Errorf("filename is required")
	}

	// Validate filename length
	if len(att.Filename) > 255 {
		return fmt.Errorf("filename too long")
	}

	// Validate size
	if att.Size <= 0 {
		return fmt.Errorf("invalid attachment size")
	}

	if att.Size > s.maxAttachmentSize {
		return fmt.Errorf("attachment size exceeds limit of %d bytes", s.maxAttachmentSize)
	}

	// Validate data
	if att.Data == "" {
		return fmt.Errorf("attachment data is required")
	}

	// Check if data can be decoded
	if _, err := base64.StdEncoding.DecodeString(att.Data); err != nil {
		return fmt.Errorf("invalid base64 data: %w", err)
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(att.Filename))

	// Dangerous extensions
	dangerousExtensions := []string{
		".exe", ".dll", ".bat", ".cmd", ".vbs", ".js", ".ps1", ".psm1",
		".sh", ".bash", ".zsh", ".py", ".php", ".jar", ".class", ".app",
		".scr", ".msi", ".com", ".pif", ".application", ".gadget",
		".msc", ".msp", ".hta", ".cpl", ".msh", ".msh1", ".msh2",
		".mshxml", ".msh1xml", ".msh2xml", ".scf", ".lnk", ".inf",
		".reg", ".docm", ".dotm", ".xlsm", ".xltm", ".xlam", ".pptm",
		".potm", ".ppam", ".ppsm", ".sldm",
	}

	for _, dangerous := range dangerousExtensions {
		if ext == dangerous {
			return fmt.Errorf("dangerous file type: %s", ext)
		}
	}

	// Validate content type
	if att.ContentType == "" {
		att.ContentType = mime.TypeByExtension(ext)
	}
	if att.ContentType == "" {
		att.ContentType = "application/octet-stream"
	}

	// Reject certain content types
	blockedContentTypes := []string{
		"application/x-msdownload",
		"application/x-ms-installer",
		"application/x-dosexec",
		"application/x-executable",
		"application/x-shellscript",
	}

	for _, blocked := range blockedContentTypes {
		if strings.HasPrefix(att.ContentType, blocked) {
			return fmt.Errorf("blocked content type: %s", att.ContentType)
		}
	}

	return nil
}

func (s *emailService) scanForViruses(data []byte) (bool, error) {
	// TODO: Implement actual virus scanning integration
	// This could integrate with ClamAV, VirusTotal API, or other scanners

	// Basic heuristic checks
	if len(data) == 0 {
		return true, nil // Empty file is safe
	}

	// Check for executable signatures
	executableSignatures := [][]byte{
		{0x4D, 0x5A},             // DOS executable
		{0x7F, 0x45, 0x4C, 0x46}, // ELF executable
		{0xCA, 0xFE, 0xBA, 0xBE}, // Java class
		{0xFE, 0xED, 0xFA, 0xCE}, // Mach-O
		{0xFE, 0xED, 0xFA, 0xCF}, // Mach-O 64-bit
		{0xCE, 0xFA, 0xED, 0xFE}, // Mach-O little endian
		{0xCF, 0xFA, 0xED, 0xFE}, // Mach-O 64-bit little endian
	}

	for _, sig := range executableSignatures {
		if bytes.HasPrefix(data, sig) {
			s.logger.Warn("Executable file detected",
				zap.String("signature", fmt.Sprintf("%X", sig)))
			// Not necessarily a virus, but potentially dangerous
			// In production, this would be passed to actual virus scanner
		}
	}

	// Check for script headers
	scriptHeaders := []string{
		"#!/bin/bash",
		"#!/bin/sh",
		"#!/usr/bin/env python",
		"#!/usr/bin/env php",
		"#!/usr/bin/env perl",
		"#!/usr/bin/env ruby",
		"<%@",
		"<?php",
		"<script",
	}

	contentStr := string(data[:min(1024, len(data))])
	for _, header := range scriptHeaders {
		if strings.HasPrefix(contentStr, header) {
			s.logger.Warn("Script file detected", zap.String("header", header))
			// Not necessarily a virus, but potentially dangerous
		}
	}

	// For now, accept all files (in production, implement real scanning)
	return true, nil
}

func (s *emailService) storeAttachment(ctx context.Context, userID uuid.UUID, att AttachmentInfo) (uuid.UUID, int64, error) {
	// Validate attachment
	if err := s.validateAttachment(att); err != nil {
		return uuid.Nil, 0, fmt.Errorf("attachment validation failed: %w", err)
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("invalid attachment data: %w", err)
	}

	// Verify size matches
	if int64(len(data)) != att.Size {
		return uuid.Nil, 0, fmt.Errorf("attachment size mismatch: expected %d, got %d", att.Size, len(data))
	}

	// Scan for viruses
	clean, err := s.scanForViruses(data)
	if err != nil {
		return uuid.Nil, 0, fmt.Errorf("virus scan failed: %w", err)
	}
	if !clean {
		return uuid.Nil, 0, fmt.Errorf("virus detected in attachment")
	}

	// Generate content hash for deduplication
	hash := sha256.Sum256(data)
	contentHash := hex.EncodeToString(hash[:])

	// Check if file already exists
	if existingID, err := s.storageService.FindByHash(ctx, contentHash); err == nil {
		s.logger.Debug("Duplicate attachment found, reusing",
			zap.String("filename", att.Filename),
			zap.String("content_hash", contentHash[:16]))
		return existingID, int64(len(data)), nil
	}

	attachmentID := uuid.New()

	// Store new file
	if err := s.storageService.Store(ctx, attachmentID.String(), bytes.NewReader(data)); err != nil {
		return uuid.Nil, 0, fmt.Errorf("failed to store file: %w", err)
	}

	// Store metadata
	metadata := map[string]interface{}{
		"user_id":       userID.String(),
		"filename":      att.Filename,
		"content_type":  att.ContentType,
		"size":          len(data),
		"content_hash":  contentHash,
		"uploaded_at":   time.Now().UTC(),
		"is_inline":     att.IsInline,
		"original_name": filepath.Base(att.Filename),
	}

	if err := s.storageService.SetMetadata(ctx, attachmentID.String(), metadata); err != nil {
		s.logger.Error("Failed to store attachment metadata", zap.Error(err))
		// Clean up stored file if metadata fails
		s.storageService.Delete(ctx, attachmentID.String())
		return uuid.Nil, 0, fmt.Errorf("failed to store metadata: %w", err)
	}

	return attachmentID, int64(len(data)), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
