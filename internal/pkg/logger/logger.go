package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/natefinch/lumberjack"
)

const (
	LevelDebug slog.Level = -4
	LevelInfo  slog.Level = 0
	LevelWarn  slog.Level = 4
	LevelError slog.Level = 8
)

var out = slog.Default()

func Std() *slog.Logger {
	return slog.Default()
}

func CreateFileWriter(filePath string) io.Writer {
	return &lumberjack.Logger{
		Filename:   filePath, // 日志文件的位置
		MaxSize:    100,      // 文件最大尺寸（以MB为单位）
		MaxBackups: 3,        // 保留的最大旧文件数量
		MaxAge:     7,        // 保留旧文件的最大天数
		Compress:   true,     // 是否压缩/归档旧文件
		LocalTime:  true,     // 使用本地时间创建时间戳
	}
}

func formatTimeAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05.000"))
	}
	return a
}

// Init 初始化日志；console 为 true 时同时输出到标准输出（便于本地调试）。
func Init(filePath string, level slog.Level, topic string, console bool) {
	opts := &slog.HandlerOptions{
		AddSource:   true,
		Level:       level,
		ReplaceAttr: formatTimeAttr,
	}
	fileHandler := slog.NewJSONHandler(CreateFileWriter(filePath), opts)
	var handler slog.Handler = fileHandler
	if console {
		handler = &multiHandler{handlers: []slog.Handler{
			fileHandler,
			slog.NewTextHandler(os.Stdout, opts),
		}}
	}
	out = slog.New(handler).With("topic", topic)
}

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range m.handlers {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: hs}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	hs := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		hs[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: hs}
}

func InfofContext(ctx context.Context, format string, args ...any) {
	logf(ctx, slog.LevelInfo, format, args...)
}

func Infof(format string, args ...any) {
	logf(context.Background(), slog.LevelInfo, format, args...)
}

func WarnfContext(ctx context.Context, format string, args ...any) {
	logf(ctx, slog.LevelWarn, format, args...)
}

func Warnf(format string, args ...any) {
	logf(context.Background(), slog.LevelWarn, format, args...)
}

func ErrorfContext(ctx context.Context, format string, args ...any) {
	logf(ctx, slog.LevelError, format, args...)
}

func Errorf(format string, args ...any) {
	logf(context.Background(), slog.LevelError, format, args...)
}

func logf(ctx context.Context, level slog.Level, format string, args ...any) {
	if !out.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])

	if len(args) > 0 {
		format = fmt.Sprintf(format, args...)
	}

	r := slog.NewRecord(time.Now(), level, format, pcs[0])

	traceId := ctx.Value("trace_id")
	if traceId != nil {
		r.Add("trace_id", traceId)
	}

	_ = out.Handler().Handle(ctx, r)
}

func WithFields(level slog.Level, msg string, fields any) {
	if !out.Enabled(context.Background(), level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add("extra", fields)

	_ = out.Handler().Handle(context.Background(), r)
}

func ErrorWithFields(msg string, err error, fields any) {
	ctx := context.Background()

	if !out.Enabled(ctx, slog.LevelError) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(2, pcs[:])

	r := slog.NewRecord(time.Now(), slog.LevelError, msg, pcs[0])
	r.Add("error", err.Error())
	r.Add("extra", fields)

	_ = out.Handler().Handle(ctx, r)
}
