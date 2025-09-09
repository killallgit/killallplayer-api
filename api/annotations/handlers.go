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
// @Param        id path int64 true "Episode ID"
// @Param        annotation body models.Annotation true "Annotation data (label, start_time, end_time)"
// @Success      201 {object} models.Annotation "Created annotation"
// @Failure      400 {object} types.ErrorResponse "Invalid request"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/annotations [post]
func CreateAnnotation(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse Podcast Index ID (int64) from URL
		podcastIndexID, ok := types.ParseInt64Param(c, "id")
		if !ok {
			return // Error response already sent by utility
		}

		// Fetch episode by PodcastIndexID to get database ID
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), podcastIndexID)
		if err != nil {
			types.SendNotFound(c, "Episode not found")
			return
		}

		// Parse request body using utility function
		var annotation models.Annotation
		if !types.BindJSONOrError(c, &annotation) {
			return // Error response already sent by utility
		}

		// Set episode ID (database ID)
		annotation.EpisodeID = episode.ID

		// Create annotation using service
		if err := deps.AnnotationService.CreateAnnotation(c.Request.Context(), &annotation); err != nil {
			// Check if it's a validation error
			if err.Error() == "Label is required" ||
				err.Error() == "Start time must be before end time" ||
				err.Error() == "Episode ID is required" {
				types.SendBadRequest(c, err.Error())
			} else {
				types.SendInternalError(c, "Failed to create annotation")
			}
			return
		}

		types.SendCreated(c, annotation)
	}
}

// GetAnnotations retrieves all annotations for an episode
// @Summary      Get annotations for episode
// @Description  Retrieve all annotations (labeled time segments) for a specific episode, ordered by start time
// @Tags         annotations
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode ID"
// @Success      200 {object} object{annotations=[]models.Annotation} "List of annotations"
// @Failure      400 {object} types.ErrorResponse "Invalid episode ID"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/annotations [get]
func GetAnnotations(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse Podcast Index ID (int64) from URL
		podcastIndexID, ok := types.ParseInt64Param(c, "id")
		if !ok {
			return // Error response already sent by utility
		}

		// Fetch episode by PodcastIndexID to get database ID
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), podcastIndexID)
		if err != nil {
			types.SendNotFound(c, "Episode not found")
			return
		}

		// Get annotations using service
		annotations, err := deps.AnnotationService.GetAnnotationsByEpisodeID(c.Request.Context(), episode.ID)
		if err != nil {
			types.SendInternalError(c, "Failed to retrieve annotations")
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
// @Failure      400 {object} types.ErrorResponse "Invalid request"
// @Failure      404 {object} types.ErrorResponse "Annotation not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/annotations/{id} [put]
func UpdateAnnotation(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		annotationIDStr := c.Param("id")

		// Parse annotation ID
		annotationID, err := strconv.ParseUint(annotationIDStr, 10, 32)
		if err != nil {
			types.SendBadRequest(c, "Invalid annotation ID")
			return
		}

		// Parse request body
		var updateData models.Annotation
		if !types.BindJSONOrError(c, &updateData) {
			return // Error response already sent by utility
		}

		// Update annotation using service
		annotation, err := deps.AnnotationService.UpdateAnnotation(
			c.Request.Context(),
			uint(annotationID),
			updateData.Label,
			updateData.StartTime,
			updateData.EndTime,
		)

		if err != nil {
			// Check if it's a validation error or not found
			if err.Error() == "Label is required" ||
				err.Error() == "Start time must be before end time" {
				types.SendBadRequest(c, err.Error())
			} else if err.Error() == "annotation not found" {
				types.SendNotFound(c, "Annotation not found")
			} else {
				types.SendInternalError(c, "Failed to update annotation")
			}
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
// @Failure      400 {object} types.ErrorResponse "Invalid annotation ID"
// @Failure      404 {object} types.ErrorResponse "Annotation not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/annotations/{id} [delete]
func DeleteAnnotation(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		annotationIDStr := c.Param("id")

		// Parse annotation ID
		annotationID, err := strconv.ParseUint(annotationIDStr, 10, 32)
		if err != nil {
			types.SendBadRequest(c, "Invalid annotation ID")
			return
		}

		// Delete annotation using service
		if err := deps.AnnotationService.DeleteAnnotation(c.Request.Context(), uint(annotationID)); err != nil {
			if err.Error() == "annotation not found" {
				types.SendNotFound(c, "Annotation not found")
			} else {
				types.SendInternalError(c, "Failed to delete annotation")
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Annotation deleted successfully"})
	}
}
