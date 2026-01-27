package notificationsdelivery

import "errors"

var ErrAlreadySent = errors.New("notification already sent")
var ErrInProgress = errors.New("notification send already in progress")
