package util

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type EncoderFunc func(w io.Writer, data any) error
type DecoderFunc func(data []byte, v any) error

type RawEditWfFunc func(editor string, data any) (rawData []byte, err error)

type EditorConfig struct {
	// The data struct to be encoded, edited, and decoded back into.
	// This MUST be a pointer to update the original data.
	TargetDataStructure any

	// File parameters
	FileName string
	Editor   string

	// Raw bytes "access point", if not nil file content will reflect here
	RawBytesAp *[]byte

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

func setupTempFile(cfg *EditorConfig) (*os.File, error) {
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

func openEditor(file *os.File, cfg *EditorConfig) error {
	// Use cfg.Editor
	if err := OpenFileInEditor(file, cfg.Editor); err != nil {
		return fmt.Errorf("external editor failed: %w", err)
	}
	return nil
}

func readBackAndDecode(file *os.File, cfg *EditorConfig) error {
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

	if cfg.RawBytesAp != nil {
		*cfg.RawBytesAp = modifiedBytes
	}

	if cfg.TargetDataStructure != nil {
		return cfg.Decoder(modifiedBytes, cfg.TargetDataStructure)
	}

	return nil
}

func EditorWorkflow(cfg *EditorConfig) error {
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

func JsonEncoder(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func JsonDecoder(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func XmlEncoder(w io.Writer, data any) error {
	enc := xml.NewEncoder(w)
	return enc.Encode(data)
}

func XmlDecoder(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func TomlEncoder(w io.Writer, data any) error {
	return toml.NewEncoder(w).Encode(data)
}

func TomlDecoder(data []byte, v any) error {
	return toml.Unmarshal(data, v)
}

func EditToml(data any, editor string) error {
	cfg := &EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.req.toml",
		Editor:              editor,
		Encoder:             TomlEncoder,
		Decoder:             TomlDecoder,
	}

	return EditorWorkflow(cfg)
}

func EditJSON(data any, editor string) error {
	cfg := &EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.payload.json",
		Editor:              editor,
		Encoder:             JsonEncoder,
		Decoder:             JsonDecoder,
	}

	return EditorWorkflow(cfg)
}

func EditXML(data any, editor string) error {
	cfg := &EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.payload.xml",
		Editor:              editor,
		Encoder:             XmlEncoder,
		Decoder:             XmlDecoder,
	}

	return EditorWorkflow(cfg)
}

func EditTextRawWf(editor string, data any) ([]byte, error) {
	var rawBytes []byte
	cfg := &EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.payload.txt",
		Editor:              editor,
		Encoder:             RawEncoder, // Use RawEncoder to avoid "null" or quotes
		Decoder:             RawDecoder, // No-op decoder
		RawBytesAp:          &rawBytes,
	}

	err := EditorWorkflow(cfg)
	return rawBytes, err
}

// The json will not be decoded into a target ds, instead raw bytes will be returned.
func EditJsonRawWf(editor string, data any) ([]byte, error) {
	var rawBytes []byte
	cfg := &EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.payload.json",
		Editor:              editor,
		Encoder:             RawEncoder, // Use RawEncoder to avoid "null" or quotes
		Decoder:             RawDecoder, // No-op decoder
		RawBytesAp:          &rawBytes,
	}

	err := EditorWorkflow(cfg)
	return rawBytes, err
}

func EditXMLRawWf(editor string, data any) ([]byte, error) {
	var rawBytes []byte
	cfg := &EditorConfig{
		TargetDataStructure: data,
		FileName:            "reql-reqs.payload.xml",
		Editor:              editor,
		Encoder:             RawEncoder,
		Decoder:             RawDecoder,
		RawBytesAp:          &rawBytes,
	}

	err := EditorWorkflow(cfg)
	return rawBytes, err
}

func ToIndentedPayload(data any) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	var raw []byte
	switch v := data.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return json.MarshalIndent(v, "", "  ")
	}

	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return raw, nil
	}

	var buf bytes.Buffer
	firstChar := trimmed[0]

	// Try JSON
	if firstChar == '{' || firstChar == '[' {
		if err := json.Indent(&buf, raw, "", "  "); err == nil {
			return buf.Bytes(), nil
		}
	}

	// Try XML
	if firstChar == '<' {
		decoder := xml.NewDecoder(bytes.NewReader(raw))
		encoder := xml.NewEncoder(&buf)
		encoder.Indent("", "  ")

		validXML := true
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				validXML = false
				break
			}
			encoder.EncodeToken(token)
		}
		encoder.Flush()
		if validXML {
			return buf.Bytes(), nil
		}
	}

	// Fallback for plain text or invalid JSON/XML
	return raw, nil
}

func RawEncoder(w io.Writer, data any) error {
	formatted, err := ToIndentedPayload(data)
	if err != nil {
		return err
	}
	_, err = w.Write(formatted)
	return err
}

func RawDecoder(data []byte, v any) error {
	return nil
}

type ResponseData struct {
	Status     string              `toml:"status"`
	StatusCode int                 `toml:"status_code"`
	Proto      string              `toml:"protocol"`
	Headers    map[string][]string `toml:"headers"`
	Body       string              `toml:"body"`
}

func EditResponseInToml(resp *http.Response, editor string) (*ResponseData, error) {
	if resp == nil {
		return nil, fmt.Errorf("no response to edit")
	}

	bodyBytes, err := ReadAndResetIoCloser(&resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	resp.Body.Close()

	indentedBody, _ := ToIndentedPayload(bodyBytes)

	data := &ResponseData{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Proto:      resp.Proto,
		Headers:    resp.Header,
		Body:       string(indentedBody),
	}

	cfg := &EditorConfig{
		TargetDataStructure: data,
		FileName:            "http-response.toml",
		Editor:              editor,
		Encoder:             TomlEncoder,
		Decoder:             TomlDecoder,
	}

	if err := EditorWorkflow(cfg); err != nil {
		return nil, err
	}

	return data, nil
}
