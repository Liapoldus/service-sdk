package models

import "errors"

// ErrNotReady is returned by lifecycle queries before a configuration
// generation has been applied and acknowledged.
var ErrNotReady = errors.New("service has no acknowledged configuration generation")
