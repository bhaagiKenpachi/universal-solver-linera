package solver

import (
	"log"
	"os"
)

var (
	// Logger is a global logger instance for the solver package
	Logger *log.Logger
)

// InitLogger initializes the logger with custom configuration
func InitLogger() {
	// Create a logger with timestamp, file location, and line number
	Logger = log.New(os.Stdout, "SOLVER: ", log.Ldate|log.Ltime|log.Lshortfile)
}

// init ensures the logger is initialized when the package is imported
func init() {
	if Logger == nil {
		InitLogger()
	}
} 