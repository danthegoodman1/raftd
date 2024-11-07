package http_server

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

func (s *HTTPServer) Lookup(c *CustomContext) error {
	panic("todo")
}

func (s *HTTPServer) Update(c *CustomContext) error {
	panic("todo")
}

func (s *HTTPServer) CreateSnapshot(c *CustomContext) error {
	panic("todo")
}

func (s *HTTPServer) ReadSnapshot(c *CustomContext) error {
	panic("todo")
}

type RecruitRequest struct {
	ReplicaAddr string
	ReplicaID   uint64
	ShardID     uint64
}

// RecruitReplica a new raft member into the cluster
func (s *HTTPServer) RecruitReplica(c *CustomContext) error {
	ctx := c.Request().Context()
	var body RecruitRequest
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	err := s.manager.RecruitReplica(ctx, body.ReplicaID, body.ShardID, body.ReplicaAddr)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusAccepted)
}

type RemoveRequest struct {
	ReplicaID uint64
	ShardID   uint64
}

// RemoveReplica a new raft member into the cluster
func (s *HTTPServer) RemoveReplica(c *CustomContext) error {
	ctx := c.Request().Context()
	var body RemoveRequest
	if err := ValidateRequest(c, &body); err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	err := s.manager.RemoveReplica(ctx, body.ReplicaID, body.ShardID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusAccepted)
}
