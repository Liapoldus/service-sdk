package models

import "errors"

var ErrReplicaIdentityMismatch = errors.New("service SDK replica identity does not match its certificate")
