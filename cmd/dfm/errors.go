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
type ErrorHandler func(err FileError) error

// Retry is used by ErrorHandler to signal to dfm to attempt the file operation
// again. The type cast is to suppress golint complaining about the variable not
// being named ErrRetry.
var Retry = errors.New("retry this file").(error)

// FileError represents any error dfm encountered while managing files.
type FileError interface {
	Message() string
	Filename() string
	Error() string
	Cause() error
}

type fileErrorImpl struct {
	message  string
	filename string
	cause    error
}

// NewFileError creates a new FileError for the provided file.
func NewFileError(filename string, message string) FileError {
	return &fileErrorImpl{
		message:  message,
		filename: filename,
	}
}

// NewFileErrorf creates a new FileError for the provided file with a format
// string.
func NewFileErrorf(filename string, message string, args ...interface{}) FileError {
	return &fileErrorImpl{
		message:  fmt.Sprintf(message, args...),
		filename: filename,
	}
}

// WrapFileError takes an existing error and creates a new FileError for the
// given file.
func WrapFileError(cause error, filename string) FileError {
	return &fileErrorImpl{
		message:  cause.Error(),
		filename: filename,
		cause:    cause,
	}
}

func (err *fileErrorImpl) Message() string {
	return err.message
}

func (err *fileErrorImpl) Filename() string {
	return err.filename
}

func (err *fileErrorImpl) Error() string {
	return fmt.Sprintf("%s: %s", err.filename, err.message)
}

// Cause is the underlying cause of the error
func (err *fileErrorImpl) Cause() error {
	return err.cause
}
