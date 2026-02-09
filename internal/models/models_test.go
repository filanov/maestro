package models_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/filanov/maestro/internal/models"
)

var _ = Describe("Models", func() {
	Describe("GenerateAgentID", func() {
		It("should generate deterministic UUID from cluster ID and hostname", func() {
			clusterID := "cluster-123"
			hostname := "worker-01"

			id1 := models.GenerateAgentID(clusterID, hostname)
			id2 := models.GenerateAgentID(clusterID, hostname)

			Expect(id1).To(Equal(id2))
			Expect(id1).NotTo(BeEmpty())
		})

		It("should generate different UUIDs for different hostnames", func() {
			clusterID := "cluster-123"

			id1 := models.GenerateAgentID(clusterID, "worker-01")
			id2 := models.GenerateAgentID(clusterID, "worker-02")

			Expect(id1).NotTo(Equal(id2))
		})

		It("should generate different UUIDs for different cluster IDs", func() {
			hostname := "worker-01"

			id1 := models.GenerateAgentID("cluster-123", hostname)
			id2 := models.GenerateAgentID("cluster-456", hostname)

			Expect(id1).NotTo(Equal(id2))
		})

		It("should generate valid UUID format", func() {
			id := models.GenerateAgentID("cluster-123", "worker-01")

			Expect(id).To(MatchRegexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))
		})
	})
})
