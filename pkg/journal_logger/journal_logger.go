package journal_logger

import (
	"context"
	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
	"github.com/coreos/go-systemd/v22/journal"
	"log/slog"
	"sync"
)

func mapLevelToPriority(level slog.Level) journal.Priority {
	switch level {
	case slog.LevelDebug:
		return journal.PriDebug
	case slog.LevelInfo:
		return journal.PriInfo
	case slog.LevelWarn:
		return journal.PriWarning
	case slog.LevelError:
		return journal.PriErr
	//case slog.Level:
	//	return journal.PriCrit
	default:
		return journal.PriInfo
	}
}

type handler struct {
	next            slog.Handler
	writeLock       *sync.Mutex
	currentPriority journal.Priority
	raw             bool
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, record slog.Record) error {
	h.writeLock.Lock()
	defer h.writeLock.Unlock()
	h.currentPriority = mapLevelToPriority(record.Level)
	return h.next.Handle(ctx, record)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h.next.WithAttrs(attrs)
}

func (h *handler) WithGroup(name string) slog.Handler {
	return h.next.WithGroup(name)
}

func (h *handler) Write(p []byte) (n int, err error) {
	var stringData string
	if h.raw {
		stringData = string(p[3:])
	} else {
		stringData = string(p)
	}

	if err := journal.Send(stringData, h.currentPriority, nil); err != nil {
		return 0, &motmedelErrors.InputError{
			Message: "An error occurred when writing.",
			Cause:   err,
			Input:   stringData,
		}
	}

	return len(p), nil
}

func NewJsonHandler(handlerOptions *slog.HandlerOptions) slog.Handler {
	h := &handler{writeLock: &sync.Mutex{}}
	h.next = slog.NewJSONHandler(h, handlerOptions)
	return h
}

func rawReplaceAttr(groups []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.MessageKey {
		attr.Key = ""
	} else {
		attr = slog.Any("", nil)
	}

	return attr
}

func NewRawHandler(level slog.Leveler) slog.Handler {
	h := &handler{writeLock: &sync.Mutex{}, raw: true}
	h.next = slog.NewTextHandler(h, &slog.HandlerOptions{ReplaceAttr: rawReplaceAttr, Level: level})

	return h
}
