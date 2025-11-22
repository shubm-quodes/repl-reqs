package util

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type EncoderFunc func(w io.Writer, data any) error
type DecoderFunc func(data []byte, v any) error

type EditorConfig struct {
	// The data struct to be encoded, edited, and decoded back into.
	// This MUST be a pointer to update the original data.
	TargetDataStructure any

	// File parameters
	FileName string
	Editor   string

	// Serialization functions
	Encoder EncoderFunc
	Decoder DecoderFunc
}

func NewTempFile(fileName string) (*os.File, error) {
	baseName := filepath.Base(fileName)
	ext := filepath.Ext(baseName)

	prefix := baseName[:len(baseName)-len(ext)]
	pattern := prefix + "*" + ext
	return os.CreateTemp("", pattern)
}

func OpenFileInEditor(file *os.File, editor string) error {
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file before editor: %w", err)
	}
	cmd := exec.Command(editor, file.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupTempFile(cfg EditorConfig) (*os.File, error) {
	file, err := NewTempFile(cfg.FileName)
	if err != nil {
		return nil, err
	}

	if err := cfg.Encoder(file, cfg.TargetDataStructure); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to encode data to file: %w", err)
	}

	return file, nil
}

func openEditor(file *os.File, cfg EditorConfig) error {
	// Use cfg.Editor
	if err := OpenFileInEditor(file, cfg.Editor); err != nil {
		return fmt.Errorf("external editor failed: %w", err)
	}
	return nil
}

func readBackAndDecode(file *os.File, cfg EditorConfig) error {
	filePath := file.Name()

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close stale file handle: %w", err)
	}

	rereadFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to re-open file after editing: %w", err)
	}
	defer rereadFile.Close()

	modifiedBytes, err := io.ReadAll(rereadFile)
	if err != nil {
		return fmt.Errorf("failed to read modified file: %w", err)
	}

	return cfg.Decoder(modifiedBytes, cfg.TargetDataStructure)
}

func EditorWorkflow(cfg EditorConfig) error {
	file, err := setupTempFile(cfg)
	if err != nil {
		return err
	}
	defer func() {
		os.Remove(file.Name()) // Clean up the temporary file
	}()

	// Block and Wait -
	if err := openEditor(file, cfg); err != nil {
		return err
	}

	if err := readBackAndDecode(file, cfg); err != nil {
		return err
	}

	return nil
}

func TomlEncoder(w io.Writer, data any) error {
	return toml.NewEncoder(w).Encode(data)
}

func TomlDecoder(data []byte, v any) error {
	return toml.Unmarshal(data, v)
}

func EditToml(data any, editor string) error {
	cfg := EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.req.toml",
		Editor:              editor,
		Encoder:             TomlEncoder,
		Decoder:             TomlDecoder,
	}

	return EditorWorkflow(cfg)
}

func EditJSON(data any, editor string) error {
	cfg := EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.payload.json",
		Editor:              editor,
		Encoder:             TomlEncoder,
		Decoder:             TomlDecoder,
	}

	return EditorWorkflow(cfg)
}

func EditXML(data any, editor string) error {
	cfg := EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.payload.xml",
		Editor:              editor,
		Encoder:             TomlEncoder,
		Decoder:             TomlDecoder,
	}

	return EditorWorkflow(cfg)
}
