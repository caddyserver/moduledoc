package moduledoc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/tools/go/packages"
)

type loggerContextKey struct{}

func packageErrorKindName(kind packages.ErrorKind) string {
	switch kind {
	case packages.UnknownError:
		return "unknown"
	case packages.ListError:
		return "list"
	case packages.ParseError:
		return "parse"
	case packages.TypeError:
		return "type"
	default:
		return fmt.Sprintf("unknown(%d)", kind)
	}
}

var defaultLogger = zap.New(zapcore.NewCore(
	zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
	zapcore.Lock(os.Stderr), zap.InfoLevel,
)).Named("moduledoc")

// WithLogger attaches an operation-scoped logger to ctx. Include correlation
// fields such as request_id and account_id on the logger before calling this.
// The logger is never stored on the shared Driver.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

func operationContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func contextLogger(ctx context.Context) *zap.Logger {
	if logger, ok := operationContext(ctx).Value(loggerContextKey{}).(*zap.Logger); ok && logger != nil {
		return logger
	}
	return defaultLogger
}

func logOperation(ctx context.Context, operation string, fields ...zap.Field) (context.Context, func(error)) {
	ctx = operationContext(ctx)
	logger := contextLogger(ctx).Named(operation).With(fields...)
	ctx = WithLogger(ctx, logger)
	start := time.Now()
	logger.Debug("operation started")
	return ctx, func(err error) {
		fields := []zap.Field{zap.Duration("duration", time.Since(start))}
		if err != nil {
			logger.Error("operation failed", append(fields, zap.Error(err))...)
		} else {
			logger.Debug("operation completed", fields...)
		}
	}
}

func runGoCommand(ctx context.Context, cmd *exec.Cmd) error {
	logger := contextLogger(ctx).With(zap.Strings("command", cmd.Args))
	start := time.Now()
	logger.Debug("Go command started")
	var diagnostics commandDiagnostics
	stdout := commandOutput{logger: logger.With(zap.String("stream", "stdout"))}
	stderr := commandOutput{logger: logger.With(zap.String("stream", "stderr"))}
	cmd.Stdout = io.MultiWriter(&stdout, &diagnostics)
	cmd.Stderr = io.MultiWriter(&stderr, &diagnostics)
	err := cmd.Run()
	stdout.flush()
	stderr.flush()
	fields := []zap.Field{zap.Duration("duration", time.Since(start))}
	if err != nil {
		logger.Error("Go command failed", append(fields,
			zap.Error(err), zap.ByteString("diagnostics", diagnostics.tail), zap.Bool("diagnostics_truncated", diagnostics.truncated))...)
		return err
	}
	logger.Debug("Go command completed", fields...)
	return nil
}

type commandDiagnostics struct {
	mu        sync.Mutex
	tail      []byte
	truncated bool
}

func (d *commandDiagnostics) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	const limit = 16 * 1024
	n := len(p)
	if n > limit {
		p = p[n-limit:]
		d.truncated = true
	}
	if excess := len(d.tail) + len(p) - limit; excess > 0 {
		copy(d.tail, d.tail[excess:])
		d.tail = d.tail[:len(d.tail)-excess]
		d.truncated = true
	}
	d.tail = append(d.tail, p...)
	return n, nil
}

type commandOutput struct {
	logger  *zap.Logger
	pending []byte
}

func (w *commandOutput) Write(p []byte) (int, error) {
	const maxEntryBytes = 16 * 1024
	n := len(p)
	for len(p) > 0 {
		size := min(len(p), maxEntryBytes-len(w.pending))
		if newline := bytes.IndexByte(p[:size], '\n'); newline >= 0 {
			size = newline + 1
		}
		w.pending = append(w.pending, p[:size]...)
		p = p[size:]
		if w.pending[len(w.pending)-1] == '\n' {
			w.flush()
		} else if len(w.pending) == maxEntryBytes {
			size := len(w.pending)
			start := size - 1
			for start > size-utf8.UTFMax && !utf8.RuneStart(w.pending[start]) {
				start--
			}
			if !utf8.FullRune(w.pending[start:]) {
				size = start
			}
			w.emit(size)
		}
	}
	return n, nil
}

func (w *commandOutput) flush() {
	if len(w.pending) == 0 {
		return
	}
	w.emit(len(w.pending))
}

func (w *commandOutput) emit(size int) {
	w.logger.Info("Go command output", zap.String("output", string(w.pending[:size])))
	copy(w.pending, w.pending[size:])
	w.pending = w.pending[:len(w.pending)-size]
}
