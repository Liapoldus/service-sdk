package models

import "errors"

var ErrReplicaIdentityConflict = errors.New("service SDK replica incarnation conflicts with Core state")
