package models

import "errors"

var ErrReplicaLeaseExpired = errors.New("service SDK replica lease expired")
