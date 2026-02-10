package dto

import (
	"fmt"
	"time"

	"github.com/filanov/maestro/internal/models"
)

type ClusterResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateClusterRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (r *CreateClusterRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(r.Name) > 255 {
		return fmt.Errorf("name must be less than 255 characters")
	}
	return nil
}

func ClusterToResponse(cluster *models.Cluster) ClusterResponse {
	return ClusterResponse{
		ID:          cluster.ID,
		Name:        cluster.Name,
		Description: cluster.Description,
		CreatedAt:   cluster.CreatedAt,
		UpdatedAt:   cluster.UpdatedAt,
	}
}

func ClustersToResponse(clusters []*models.Cluster) []ClusterResponse {
	result := make([]ClusterResponse, len(clusters))
	for i, cluster := range clusters {
		result[i] = ClusterToResponse(cluster)
	}
	return result
}
