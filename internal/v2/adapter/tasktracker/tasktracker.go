package tasktracker

import "github.com/danabrams/gromit/internal/v2/trackertypes"

// Type aliases — these are identical to the trackertypes originals.
type Bead = trackertypes.Bead
type TaskTrackerNextBeadRequest = trackertypes.TaskTrackerNextBeadRequest
type TaskTrackerNextBeadResponse = trackertypes.TaskTrackerNextBeadResponse
type TaskTrackerCreateBeadRequest = trackertypes.TaskTrackerCreateBeadRequest
type TaskTrackerCreateBeadResponse = trackertypes.TaskTrackerCreateBeadResponse
type TaskTrackerCloseBeadRequest = trackertypes.TaskTrackerCloseBeadRequest
type TaskTrackerCloseBeadResponse = trackertypes.TaskTrackerCloseBeadResponse
type TaskTrackerQueryBeadsRequest = trackertypes.TaskTrackerQueryBeadsRequest
type TaskTrackerQueryBeadsResponse = trackertypes.TaskTrackerQueryBeadsResponse
type TaskTracker = trackertypes.TaskTracker

// Backward-compatibility aliases retained for existing callers.
type NextBeadRequest = trackertypes.TaskTrackerNextBeadRequest
type NextBeadResponse = trackertypes.TaskTrackerNextBeadResponse
type CreateBeadRequest = trackertypes.TaskTrackerCreateBeadRequest
type CreateBeadResponse = trackertypes.TaskTrackerCreateBeadResponse
type CloseBeadRequest = trackertypes.TaskTrackerCloseBeadRequest
type CloseBeadResponse = trackertypes.TaskTrackerCloseBeadResponse
type QueryBeadsRequest = trackertypes.TaskTrackerQueryBeadsRequest
type QueryBeadsResponse = trackertypes.TaskTrackerQueryBeadsResponse
