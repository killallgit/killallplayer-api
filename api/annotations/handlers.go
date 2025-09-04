package annotations

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
)

// CreateAnnotation creates a new annotation for an episode
// @Summary      Create annotation for episode
// @Description  Create a new annotation (labeled time segment) for ML training on a specific episode
// @Tags         annotations
// @Accept       json
// @Produce      json
// @Param        id path int true "Episode ID"
// @Param        annotation body models.Annotation true "Annotation data (label, start_time, end_time)"
// @Success      201 {object} models.Annotation "Created annotation"
// @Failure      400 {object} object{error=string} "Invalid request"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/episodes/{id}/annotations [post]
func CreateAnnotation(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Parse request body
		var annotation models.Annotation
		if err := c.ShouldBindJSON(&annotation); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
			return
		}

		// Set episode ID and validate required fields
		annotation.EpisodeID = uint(episodeID)
		if annotation.Label == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Label is required"})
			return
		}
		if annotation.StartTime >= annotation.EndTime {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Start time must be before end time"})
			return
		}

		// Create annotation in database
		if err := deps.DB.Create(&annotation).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create annotation"})
			return
		}

		c.JSON(http.StatusCreated, annotation)
	}
}

// GetAnnotations retrieves all annotations for an episode
// @Summary      Get annotations for episode
// @Description  Retrieve all annotations (labeled time segments) for a specific episode, ordered by start time
// @Tags         annotations
// @Accept       json
// @Produce      json
// @Param        id path int true "Episode ID"
// @Success      200 {object} object{annotations=[]models.Annotation} "List of annotations"
// @Failure      400 {object} object{error=string} "Invalid episode ID"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/episodes/{id}/annotations [get]
func GetAnnotations(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Get annotations from database
		var annotations []models.Annotation
		if err := deps.DB.Where("episode_id = ?", uint(episodeID)).Order("start_time ASC").Find(&annotations).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve annotations"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"annotations": annotations})
	}
}

// UpdateAnnotation updates an existing annotation
// @Summary      Update annotation
// @Description  Update an existing annotation's label, start time, or end time
// @Tags         annotations
// @Accept       json
// @Produce      json
// @Param        id path int true "Annotation ID"
// @Param        annotation body models.Annotation true "Updated annotation data (label, start_time, end_time)"
// @Success      200 {object} models.Annotation "Updated annotation"
// @Failure      400 {object} object{error=string} "Invalid request"
// @Failure      404 {object} object{error=string} "Annotation not found"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/episodes/annotations/{id} [put]
func UpdateAnnotation(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		annotationIDStr := c.Param("id")

		// Parse annotation ID
		annotationID, err := strconv.ParseUint(annotationIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid annotation ID"})
			return
		}

		// Parse request body
		var updateData models.Annotation
		if err := c.ShouldBindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
			return
		}

		// Validate required fields
		if updateData.Label == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Label is required"})
			return
		}
		if updateData.StartTime >= updateData.EndTime {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Start time must be before end time"})
			return
		}

		// Update annotation in database
		result := deps.DB.Model(&models.Annotation{}).Where("id = ?", uint(annotationID)).Updates(map[string]interface{}{
			"label":      updateData.Label,
			"start_time": updateData.StartTime,
			"end_time":   updateData.EndTime,
		})

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update annotation"})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Annotation not found"})
			return
		}

		// Get updated annotation
		var annotation models.Annotation
		if err := deps.DB.First(&annotation, uint(annotationID)).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve updated annotation"})
			return
		}

		c.JSON(http.StatusOK, annotation)
	}
}

// DeleteAnnotation deletes an annotation
// @Summary      Delete annotation
// @Description  Delete an existing annotation by ID
// @Tags         annotations
// @Accept       json
// @Produce      json
// @Param        id path int true "Annotation ID"
// @Success      200 {object} object{message=string} "Annotation deleted successfully"
// @Failure      400 {object} object{error=string} "Invalid annotation ID"
// @Failure      404 {object} object{error=string} "Annotation not found"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/episodes/annotations/{id} [delete]
func DeleteAnnotation(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		annotationIDStr := c.Param("id")

		// Parse annotation ID
		annotationID, err := strconv.ParseUint(annotationIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid annotation ID"})
			return
		}

		// Delete annotation from database
		result := deps.DB.Delete(&models.Annotation{}, uint(annotationID))
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete annotation"})
			return
		}

		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Annotation not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Annotation deleted successfully"})
	}
}
