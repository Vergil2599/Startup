package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Proc es un plugin en ejecución: un proceso hijo que habla JSON-RPC 2.0
// por stdio, un mensaje JSON por línea.
type Proc struct {
	Manifest *Manifest
	cmd      *exec.Cmd
	stdin    *json.Encoder
	scanner  *bufio.Scanner
	mu       sync.Mutex // JSON-RPC secuencial sobre el pipe
	nextID   atomic.Int64
	failures atomic.Int32
	// Timeout por llamada; las llamadas LLM usan uno mayor.
	CallTimeout time.Duration
}

// MaxFailures consecutivos antes de suspender el plugin.
const MaxFailures = 3

// HandshakeTimeout limita el tiempo del plugin.describe inicial.
const HandshakeTimeout = 10 * time.Second

// Start lanza el proceso del plugin desde su directorio.
func Start(d Discovered) (*Proc, error) {
	if d.Manifest == nil {
		return nil, fmt.Errorf("plugin en %s: %s", d.Dir, d.LoadErr)
	}
	entry := d.Manifest.Plugin.Entry
	bin := entry[0]
	if !filepath.IsAbs(bin) {
		// El entry es relativo al dir del plugin, o un comando del PATH
		// (p. ej. "python3").
		if candidate := filepath.Join(d.Dir, bin); fileExists(candidate) {
			bin = candidate
		}
	}
	args := make([]string, 0, len(entry)-1)
	for _, a := range entry[1:] {
		if candidate := filepath.Join(d.Dir, a); fileExists(candidate) {
			a = candidate
		}
		args = append(args, a)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = d.Dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = nil // el ruido del plugin no contamina al host
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("no se pudo lanzar %s: %w", d.Manifest.Plugin.ID, err)
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 32<<20)
	return &Proc{
		Manifest: d.Manifest, cmd: cmd,
		stdin: json.NewEncoder(stdin), scanner: sc,
		CallTimeout: 30 * time.Second,
	}, nil
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Suspended indica si el plugin superó el máximo de fallos consecutivos.
func (p *Proc) Suspended() bool { return p.failures.Load() >= MaxFailures }

// Call ejecuta una llamada JSON-RPC con timeout y decodifica el resultado.
func (p *Proc) Call(ctx context.Context, method string, params, result any) error {
	if p.Suspended() {
		return fmt.Errorf("plugin %s suspendido tras %d fallos", p.Manifest.Plugin.ID, MaxFailures)
	}
	err := p.call(ctx, method, params, result)
	if err != nil {
		p.failures.Add(1)
		return err
	}
	p.failures.Store(0)
	return nil
}

func (p *Proc) call(ctx context.Context, method string, params, result any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.nextID.Add(1)
	if err := p.stdin.Encode(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return fmt.Errorf("plugin %s: escritura falló: %w", p.Manifest.Plugin.ID, err)
	}
	timeout := p.CallTimeout
	if dl, ok := ctx.Deadline(); ok {
		if until := time.Until(dl); until < timeout {
			timeout = until
		}
	}
	type scanResult struct {
		line []byte
		err  error
	}
	ch := make(chan scanResult, 1)
	go func() {
		for p.scanner.Scan() {
			line := append([]byte(nil), p.scanner.Bytes()...)
			if len(line) == 0 {
				continue
			}
			ch <- scanResult{line: line}
			return
		}
		err := p.scanner.Err()
		if err == nil {
			err = fmt.Errorf("el plugin cerró stdout")
		}
		ch <- scanResult{err: err}
	}()
	select {
	case <-time.After(timeout):
		return fmt.Errorf("plugin %s: timeout en %s (%s)", p.Manifest.Plugin.ID, method, timeout)
	case <-ctx.Done():
		return ctx.Err()
	case sr := <-ch:
		if sr.err != nil {
			return fmt.Errorf("plugin %s: %w", p.Manifest.Plugin.ID, sr.err)
		}
		var resp rpcResponse
		if err := json.Unmarshal(sr.line, &resp); err != nil {
			return fmt.Errorf("plugin %s: respuesta no es JSON-RPC: %w", p.Manifest.Plugin.ID, err)
		}
		if resp.ID != id {
			return fmt.Errorf("plugin %s: id de respuesta %d != %d", p.Manifest.Plugin.ID, resp.ID, id)
		}
		if resp.Error != nil {
			return fmt.Errorf("plugin %s: %s (código %d)", p.Manifest.Plugin.ID, resp.Error.Message, resp.Error.Code)
		}
		if result != nil {
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("plugin %s: resultado inválido: %w", p.Manifest.Plugin.ID, err)
			}
		}
		return nil
	}
}

// Describe hace el handshake inicial.
type DescribeResult struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	ExtensionPoints []string `json:"extension_points"`
}

// Describe pide al plugin su autodescripción y la valida contra el manifiesto.
func (p *Proc) Describe(ctx context.Context) (DescribeResult, error) {
	var d DescribeResult
	if err := p.Call(ctx, "plugin.describe", nil, &d); err != nil {
		return d, err
	}
	if d.ID != p.Manifest.Plugin.ID {
		return d, fmt.Errorf("el plugin se identifica como %q pero el manifiesto dice %q", d.ID, p.Manifest.Plugin.ID)
	}
	return d, nil
}

// Stop termina el proceso del plugin.
func (p *Proc) Stop() error {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	return p.cmd.Wait()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
