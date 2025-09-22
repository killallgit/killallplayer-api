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

		// Set episode ID and default clip status
		annotation.EpisodeID = episode.ID
		annotation.ClipStatus = "pending"

		// Check for duplicate annotations
		if isDuplicate, err := deps.AnnotationService.CheckOverlappingAnnotation(
			c.Request.Context(), episode.ID, annotation.StartTime, annotation.EndTime); err != nil {
			types.SendInternalError(c, "Failed to check for duplicates")
			return
		} else if isDuplicate {
			types.SendBadRequest(c, "Overlapping annotation already exists")
			return
		}

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

		// TODO: Trigger async clip extraction
		// go func() {
		//     ctx := context.Background()
		//     deps.AnnotationService.TriggerClipExtraction(ctx, &annotation, episode)
		// }()

		// Transform to API type and return
		apiAnnotation := types.FromModelAnnotation(&annotation)
		types.SendCreated(c, apiAnnotation)
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

		// Transform to API types and return
		apiAnnotations := types.FromModelAnnotationList(annotations)
		c.JSON(http.StatusOK, gin.H{"annotations": apiAnnotations})
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

// DeleteAnnotation deletes an annotation by UUID
// @Summary      Delete annotation by UUID
// @Description  Delete an existing annotation by UUID
// @Tags         annotations
// @Accept       json
// @Produce      json
// @Param        uuid path string true "Annotation UUID"
// @Success      200 {object} object{message=string} "Annotation deleted successfully"
// @Failure      404 {object} types.ErrorResponse "Annotation not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/annotations/{uuid} [delete]
func DeleteAnnotation(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("uuid")

		// Get annotation first to get the database ID
		annotation, err := deps.AnnotationService.GetAnnotationByUUID(c.Request.Context(), uuid)
		if err != nil {
			if err.Error() == "annotation not found" {
				types.SendNotFound(c, "Annotation not found")
			} else {
				types.SendInternalError(c, "Failed to retrieve annotation")
			}
			return
		}

		// Delete annotation using database ID
		if err := deps.AnnotationService.DeleteAnnotation(c.Request.Context(), annotation.ID); err != nil {
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

// GetAnnotationByUUID retrieves a single annotation by UUID
// @Summary      Get annotation by UUID
// @Description  Retrieve a specific annotation by its UUID, including clip status
// @Tags         annotations
// @Accept       json
// @Produce      json
// @Param        uuid path string true "Annotation UUID"
// @Success      200 {object} types.SingleAnnotationResponse "Annotation details"
// @Failure      404 {object} types.ErrorResponse "Annotation not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/annotations/{uuid} [get]
func GetAnnotationByUUID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("uuid")

		annotation, err := deps.AnnotationService.GetAnnotationByUUID(c.Request.Context(), uuid)
		if err != nil {
			if err.Error() == "annotation not found" {
				types.SendNotFound(c, "Annotation not found")
			} else {
				types.SendInternalError(c, "Failed to retrieve annotation")
			}
			return
		}

		// Transform to API type and return using existing response pattern
		apiAnnotation := types.FromModelAnnotation(annotation)
		response := types.SingleAnnotationResponse{
			BaseResponse: types.BaseResponse{Status: types.StatusOK},
			Annotation:   apiAnnotation,
		}

		types.SendSuccess(c, response)
	}
}

// UpdateAnnotationByUUID updates an existing annotation by UUID
// @Summary      Update annotation by UUID
// @Description  Update an existing annotation's label, start time, or end time using UUID
// @Tags         annotations
// @Accept       json
// @Produce      json
// @Param        uuid path string true "Annotation UUID"
// @Param        annotation body models.Annotation true "Updated annotation data (label, start_time, end_time)"
// @Success      200 {object} types.SingleAnnotationResponse "Updated annotation"
// @Failure      400 {object} types.ErrorResponse "Invalid request or overlapping annotation"
// @Failure      404 {object} types.ErrorResponse "Annotation not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/annotations/{uuid} [put]
func UpdateAnnotationByUUID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("uuid")

		// Parse request body using utility function
		var updateRequest models.Annotation
		if !types.BindJSONOrError(c, &updateRequest) {
			return // Error response already sent by utility
		}

		// Update annotation using service
		updatedAnnotation, err := deps.AnnotationService.UpdateAnnotationByUUID(
			c.Request.Context(),
			uuid,
			updateRequest.Label,
			updateRequest.StartTime,
			updateRequest.EndTime,
		)

		if err != nil {
			// Check error type for appropriate response
			errMsg := err.Error()
			if errMsg == "Label is required" ||
				errMsg == "Start time must be before end time" ||
				errMsg == "updated annotation would overlap with existing annotation" {
				types.SendBadRequest(c, errMsg)
			} else if errMsg == "annotation not found" {
				types.SendNotFound(c, "Annotation not found")
			} else {
				types.SendInternalError(c, "Failed to update annotation")
			}
			return
		}

		// TODO: Trigger async clip re-extraction if time bounds changed
		// go func() {
		//     ctx := context.Background()
		//     episode, _ := deps.EpisodeService.GetEpisodeByID(ctx, updatedAnnotation.EpisodeID)
		//     deps.AnnotationService.TriggerClipExtraction(ctx, updatedAnnotation, episode)
		// }()

		// Transform to API type and return using existing response pattern
		apiAnnotation := types.FromModelAnnotation(updatedAnnotation)
		response := types.SingleAnnotationResponse{
			BaseResponse: types.BaseResponse{Status: types.StatusOK},
			Annotation:   apiAnnotation,
		}

		types.SendSuccess(c, response)
	}
}
