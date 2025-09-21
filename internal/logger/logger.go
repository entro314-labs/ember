package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// LogLevel represents the logging level
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// OutputCategory represents different types of application output
type OutputCategory string

const (
	CategorySystem   OutputCategory = "system"   // System operations, startup, shutdown
	CategoryISO      OutputCategory = "iso"      // ISO discovery, validation, analysis
	CategoryDevice   OutputCategory = "device"   // Device detection, safety analysis
	CategoryProgress OutputCategory = "progress" // Progress updates, file operations
	CategoryError    OutputCategory = "error"    // Errors and warnings
	CategoryUser     OutputCategory = "user"     // User interactions, prompts
)

// Logger wraps slog.Logger with application-specific functionality
type Logger struct {
	*slog.Logger
	mu       sync.RWMutex
	category OutputCategory
	quiet    bool
	verbose  bool
}

// NewLogger creates a new structured logger with enhanced formatting
func NewLogger(level LogLevel, output io.Writer, quiet, verbose bool) *Logger {
	var logLevel slog.Level
	switch level {
	case LogLevelDebug:
		logLevel = slog.LevelDebug
	case LogLevelWarn:
		logLevel = slog.LevelWarn
	case LogLevelError:
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: verbose,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format time as readable timestamp
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   a.Key,
					Value: slog.StringValue(a.Value.Time().Format("15:04:05")),
				}
			}
			return a
		},
	}

	var handler slog.Handler
	if output == os.Stdout && !quiet {
		// Use enhanced text handler for better readability
		handler = NewEnhancedTextHandler(output, opts)
	} else {
		// Use JSON handler for files or quiet mode
		handler = slog.NewJSONHandler(output, opts)
	}

	return &Logger{
		Logger:   slog.New(handler),
		category: CategorySystem,
		quiet:    quiet,
		verbose:  verbose,
	}
}

// WithContext adds context information to the logger
func (l *Logger) WithContext(keyvals ...any) *Logger {
	return &Logger{
		Logger:   l.Logger.With(keyvals...),
		category: l.category,
		quiet:    l.quiet,
		verbose:  l.verbose,
	}
}

// WithCategory creates a logger for a specific output category
func (l *Logger) WithCategory(cat OutputCategory) *Logger {
	return &Logger{
		Logger:   l.Logger.With("category", string(cat)),
		category: cat,
		quiet:    l.quiet,
		verbose:  l.verbose,
	}
}

// Step logs a major step in the process with enhanced formatting
func (l *Logger) Step(step int, total int, message string, args ...any) {
	if l.quiet {
		return
	}
	formatted := fmt.Sprintf("[%d/%d] %s", step, total, message)
	l.Info(formatted, args...)
}

// Progress logs progress information in a user-friendly format
func (l *Logger) Progress(percentage float64, message string, args ...any) {
	if l.quiet {
		return
	}
	formatted := fmt.Sprintf("%.1f%% - %s", percentage, message)
	l.Info(formatted, args...)
}

// Success logs a success message with celebration emoji
func (l *Logger) Success(message string, args ...any) {
	l.Info("✅ "+message, args...)
}

// Warning logs a warning with appropriate icon
func (l *Logger) Warning(message string, args ...any) {
	l.Warn("⚠️  "+message, args...)
}

// Fatal logs a fatal error and exits the program
func (l *Logger) Fatal(msg string, args ...any) {
	l.Logger.Error(msg, args...)
	os.Exit(1)
}

// Global logger instance
var logger *Logger

// InitLogger initializes the global logger with enhanced options
func InitLogger(level LogLevel, verbose bool) {
	if verbose {
		level = LogLevelDebug
	}
	quiet := level == LogLevelError
	logger = NewLogger(level, os.Stdout, quiet, verbose)
}

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	if logger == nil {
		logger = NewLogger(LogLevelInfo, os.Stdout, false, false)
	}
	return logger
}

// EnhancedTextHandler provides better formatted output for terminal display
type EnhancedTextHandler struct {
	*slog.TextHandler
	w io.Writer
}

// NewEnhancedTextHandler creates a new enhanced text handler
func NewEnhancedTextHandler(w io.Writer, opts *slog.HandlerOptions) *EnhancedTextHandler {
	return &EnhancedTextHandler{
		TextHandler: slog.NewTextHandler(w, opts),
		w:           w,
	}
}

// Handle formats log records with enhanced visual styling
func (h *EnhancedTextHandler) Handle(ctx context.Context, r slog.Record) error {
	// For simple info messages without attributes, use clean format
	if r.Level == slog.LevelInfo && r.NumAttrs() <= 1 {
		// Check if this is a category-only log
		var hasCategory bool
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "category" {
				hasCategory = true
				return false
			}
			return true
		})

		if hasCategory || r.NumAttrs() == 0 {
			// Simple clean output for user-facing messages
			_, err := fmt.Fprintf(h.w, "%s\n", r.Message)
			return err
		}
	}

	// For everything else, use structured format
	return h.TextHandler.Handle(ctx, r)
}

// GetCategoryIcon returns an appropriate icon for the category
func GetCategoryIcon(cat OutputCategory) string {
	switch cat {
	case CategorySystem:
		return "🔧"
	case CategoryISO:
		return "💿"
	case CategoryDevice:
		return "🔌"
	case CategoryProgress:
		return "📊"
	case CategoryError:
		return "❌"
	case CategoryUser:
		return "👤"
	default:
		return "ℹ️"
	}
}
