// Package gitserver exposes a Git smart HTTP server for repositories below a
// configured project root.
package gitserver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// Handler serves repositories from ProjectRoot through Git's smart HTTP
// backend. The backend supports both upload-pack and receive-pack operations.
type Handler struct {
	ProjectRoot string
	GitBinary   string
}

// NewHandler creates a smart HTTP handler rooted at projectRoot. Repositories
// are addressed by their URL path, for example /repo.git.
func NewHandler(projectRoot string) (*Handler, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	if projectRoot == "" {
		projectRoot = root
	} else if !path.IsAbs(projectRoot) {
		projectRoot = path.Join(root, projectRoot)
	}
	return &Handler{ProjectRoot: projectRoot, GitBinary: "git"}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := validatePath(r.URL.Path); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	output, err := h.runBackend(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := writeCGIResponse(w, output); err != nil {
		return
	}
}

func (h *Handler) runBackend(r *http.Request) ([]byte, error) {
	gitBinary := h.GitBinary
	if gitBinary == "" {
		gitBinary = "git"
	}
	// The server explicitly exposes receive-pack because push is part of the
	// smart HTTP contract. Authentication and authorization belong at the HTTP
	// boundary in front of this handler.
	cmd := exec.CommandContext(r.Context(), gitBinary, "-c", "http.receivepack=true", "http-backend")
	cmd.Stdin = r.Body
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	env := append([]string{}, os.Environ()...)
	setEnv := func(key, value string) {
		prefix := key + "="
		for index, item := range env {
			if strings.HasPrefix(item, prefix) {
				env[index] = prefix + value
				return
			}
		}
		env = append(env, prefix+value)
	}
	setEnv("GIT_PROJECT_ROOT", h.ProjectRoot)
	setEnv("GIT_HTTP_EXPORT_ALL", "1")
	setEnv("PATH_INFO", r.URL.Path)
	setEnv("REQUEST_METHOD", r.Method)
	setEnv("QUERY_STRING", r.URL.RawQuery)
	setEnv("CONTENT_TYPE", r.Header.Get("Content-Type"))
	setEnv("CONTENT_LENGTH", contentLength(r))
	setEnv("REMOTE_ADDR", r.RemoteAddr)
	setEnv("SERVER_PROTOCOL", r.Proto)
	setEnv("HTTP_HOST", r.Host)
	for key, values := range r.Header {
		if len(values) == 0 {
			continue
		}
		envKey := "HTTP_" + strings.ReplaceAll(strings.ToUpper(key), "-", "_")
		setEnv(envKey, strings.Join(values, ","))
	}
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git http-backend: %w: %s", err, output.String())
	}
	return output.Bytes(), nil
}

func contentLength(r *http.Request) string {
	if r.ContentLength >= 0 {
		return strconv.FormatInt(r.ContentLength, 10)
	}
	return r.Header.Get("Content-Length")
}

func validatePath(requestPath string) error {
	if requestPath == "" || !strings.HasPrefix(requestPath, "/") {
		return fmt.Errorf("invalid repository path")
	}
	clean := path.Clean(requestPath)
	if clean != requestPath || strings.Contains(requestPath, "..") {
		return fmt.Errorf("invalid repository path")
	}
	return nil
}

func writeCGIResponse(w http.ResponseWriter, output []byte) error {
	reader := bufio.NewReader(bytes.NewReader(output))
	header, err := textproto.NewReader(reader).ReadMIMEHeader()
	if err != nil {
		return fmt.Errorf("parse git http-backend headers: %w", err)
	}
	status := http.StatusOK
	if value := header.Get("Status"); value != "" {
		fields := strings.Fields(value)
		if len(fields) == 0 {
			return fmt.Errorf("empty git http-backend status")
		}
		code, parseErr := strconv.Atoi(fields[0])
		if parseErr != nil {
			return fmt.Errorf("parse git http-backend status: %w", parseErr)
		}
		status = code
	}
	for key, values := range header {
		if strings.EqualFold(key, "Status") {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(status)
	_, err = io.Copy(w, reader)
	return err
}

func isHeaderWritten(http.ResponseWriter) bool {
	// The standard ResponseWriter does not expose this state. This function is
	// intentionally conservative; backend errors are reported before writing.
	return false
}

var _ http.Handler = (*Handler)(nil)
