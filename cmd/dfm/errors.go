package main

import (
	"errors"
	"fmt"
)

// ErrorHandler is the type of function called when dfm encounters an error with
// a particular file. The encountered error will be passed in. Dfm's behavior is
// based on the result of the handler. If the handler returns nil, dfm will
// ignore the failure and continue. If the handler returns `dfm.Retry`, dfm will
// attempt the operation again (and call the handler with the new error, if
// any). If the handler returns anything else, dfm will abort and return the
// error.
type ErrorHandler func(err *FileError) error

// Retry is used by ErrorHandler to signal to dfm to attempt the file operation
// again. The type cast is to suppress golint complaining about the variable not
// being named ErrRetry.
var Retry = errors.New("retry this file").(error)

// Skipped is used internally to signify that a file was already up to date and
// nothing needed to happen.
var Skipped = errors.New("skipped").(error)

// FileError represents any error dfm encountered while managing files.
type FileError struct {
	Message  string
	Filename string
	cause    error
}

// NewFileError creates a new FileError for the provided file.
func NewFileError(filename string, message string) *FileError {
	return &FileError{
		Message:  message,
		Filename: filename,
	}
}

// NewFileErrorf creates a new FileError for the provided file with a format
// string.
func NewFileErrorf(filename string, message string, args ...interface{}) *FileError {
	return &FileError{
		Message:  fmt.Sprintf(message, args...),
		Filename: filename,
	}
}

// WrapFileError takes an existing error and creates a new FileError for the
// given file.
func WrapFileError(cause error, filename string) *FileError {
	return &FileError{
		Message:  cause.Error(),
		Filename: filename,
		cause:    cause,
	}
}

func (err *FileError) Error() string {
	return fmt.Sprintf("%s: %s", err.Filename, err.Message)
}

// Cause is the underlying cause of the error
func (err *FileError) Cause() error {
	if err.cause == nil {
		return nil
	}
	return err.cause
}
