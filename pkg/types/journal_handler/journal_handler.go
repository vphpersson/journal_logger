package journal_handler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	"github.com/coreos/go-systemd/v22/journal"
)

var LevelToPriority = map[slog.Level]journal.Priority{
	slog.LevelDebug: journal.PriDebug,
	slog.LevelInfo:  journal.PriInfo,
	slog.LevelWarn:  journal.PriWarning,
	slog.LevelError: journal.PriErr,
}

type Handler struct {
	Next            slog.Handler
	writeLock       *sync.Mutex
	currentPriority *journal.Priority
	Raw             bool
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Next.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, record slog.Record) error {
	h.writeLock.Lock()
	defer h.writeLock.Unlock()
	priority, ok := LevelToPriority[record.Level]
	if !ok {
		priority = journal.PriInfo
	}
	*h.currentPriority = priority
	return h.Next.Handle(ctx, record)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		Next:            h.Next.WithAttrs(attrs),
		writeLock:       h.writeLock,
		currentPriority: h.currentPriority,
		Raw:             h.Raw,
	}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		Next:            h.Next.WithGroup(name),
		writeLock:       h.writeLock,
		currentPriority: h.currentPriority,
		Raw:             h.Raw,
	}
}

func (h *Handler) Write(p []byte) (n int, err error) {
	var stringData string
	if h.Raw && len(p) > 3 {
		stringData = string(p[3:])
	} else {
		stringData = string(p)
	}

	if err := journal.Send(stringData, *h.currentPriority, nil); err != nil {
		return 0, motmedelErrors.NewWithTrace(fmt.Errorf("journal send: %w", err), stringData)
	}

	return len(p), nil
}

func New(handler slog.Handler) *Handler {
	return &Handler{
		Next:            handler,
		writeLock:       &sync.Mutex{},
		currentPriority: new(journal.Priority),
	}
}

func rawReplaceAttr(groups []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.MessageKey {
		attr.Key = ""
	} else {
		attr = slog.Any("", nil)
	}

	return attr
}

func NewRaw(handlerOptions *slog.HandlerOptions) *Handler {
	if handlerOptions == nil {
		handlerOptions = &slog.HandlerOptions{}
	}

	if handlerOptions.ReplaceAttr == nil {
		handlerOptions.ReplaceAttr = rawReplaceAttr
	}

	h := &Handler{
		Raw:             true,
		writeLock:       &sync.Mutex{},
		currentPriority: new(journal.Priority),
	}
	h.Next = slog.NewTextHandler(h, handlerOptions)
	return h
}
