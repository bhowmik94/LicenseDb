// SPDX-FileCopyrightText: 2023 Kavya Shukla <kavyuushukla@gmail.com>
// SPDX-FileCopyrightText: 2023 Siemens AG
// SPDX-FileContributor: Gaurav Mishra <mishra.gaurav@siemens.com>
//
// SPDX-License-Identifier: GPL-2.0-only

package api

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/fossology/LicenseDb/pkg/db"
	"github.com/fossology/LicenseDb/pkg/models"
	"github.com/fossology/LicenseDb/pkg/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetAllObligation retrieves a list of all obligation records
//
//	@Summary		Get all active obligations
//	@Description	Get all active obligations from the service
//	@Id				GetAllObligation
//	@Tags			Obligations
//	@Accept			json
//	@Produce		json
//	@Param			active	query		bool	true	"Active obligation only"
//	@Param			page	query		int		false	"Page number"
//	@Param			limit	query		int		false	"Number of records per page"
//	@Success		200		{object}	models.ObligationResponse
//	@Failure		404		{object}	models.LicenseError	"No obligations in DB"
//	@Router			/obligations [get]
func GetAllObligation(c *gin.Context) {
	var obligations []models.Obligation
	active := c.Query("active")
	if active == "" {
		active = "true"
	}
	var parsedActive bool
	parsedActive, err := strconv.ParseBool(active)
	if err != nil {
		er := models.LicenseError{
			Status:    http.StatusBadRequest,
			Message:   "Invalid active value",
			Error:     fmt.Sprintf("Parsing failed for value '%s'", active),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusBadRequest, er)
		return
	}
	query := db.DB.Model(&models.Obligation{})
	query.Where("active = ?", parsedActive)

	_ = utils.PreparePaginateResponse(c, query, &models.ObligationResponse{})

	if err = query.Find(&obligations).Error; err != nil {
		er := models.LicenseError{
			Status:    http.StatusNotFound,
			Message:   "Obligations not found",
			Error:     err.Error(),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusNotFound, er)
		return
	}
	res := models.ObligationResponse{
		Data:   obligations,
		Status: http.StatusOK,
		Meta: &models.PaginationMeta{
			ResourceCount: len(obligations),
		},
	}

	c.JSON(http.StatusOK, res)
}

// GetObligation retrieves an active obligation record
//
//	@Summary		Get an obligation
//	@Description	Get an active based on given topic
//	@Id				GetObligation
//	@Tags			Obligations
//	@Accept			json
//	@Produce		json
//	@Param			topic	path		string	true	"Topic of the obligation"
//	@Success		200		{object}	models.ObligationResponse
//	@Failure		404		{object}	models.LicenseError	"No obligation with given topic found"
//	@Router			/obligations/{topic} [get]
func GetObligation(c *gin.Context) {
	var obligation models.Obligation
	query := db.DB.Model(&obligation)
	tp := c.Param("topic")
	if err := query.Where(models.Obligation{Topic: tp}).First(&obligation).Error; err != nil {
		er := models.LicenseError{
			Status:    http.StatusNotFound,
			Message:   fmt.Sprintf("obligation with topic '%s' not found", tp),
			Error:     err.Error(),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusNotFound, er)
		return
	}
	res := models.ObligationResponse{
		Data:   []models.Obligation{obligation},
		Status: http.StatusOK,
		Meta: &models.PaginationMeta{
			ResourceCount: 1,
		},
	}
	c.JSON(http.StatusOK, res)
}

// CreateObligation creates a new obligation record and associates it with relevant licenses.
//
//	@Summary		Create an obligation
//	@Description	Create an obligation and associate it with licenses
//	@Id				CreateObligation
//	@Tags			Obligations
//	@Accept			json
//	@Produce		json
//	@Param			obligation	body		models.ObligationPOSTRequestJSONSchema	true	"Obligation to create"
//	@Success		201			{object}	models.ObligationResponse
//	@Failure		400			{object}	models.LicenseError	"Bad request body"
//	@Failure		409			{object}	models.LicenseError	"Obligation with same body exists"
//	@Failure		500			{object}	models.LicenseError	"Unable to create obligation"
//	@Security		ApiKeyAuth
//	@Router			/obligations [post]
func CreateObligation(c *gin.Context) {
	var input models.ObligationPOSTRequestJSONSchema

	if err := c.ShouldBindJSON(&input); err != nil {
		er := models.LicenseError{
			Status:    http.StatusBadRequest,
			Message:   "invalid json body",
			Error:     err.Error(),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusBadRequest, er)
		return
	}
	s := input.Text
	hash := md5.Sum([]byte(s))
	md5hash := hex.EncodeToString(hash[:])

	obligation := models.Obligation{
		Md5:            md5hash,
		Type:           input.Type,
		Topic:          input.Topic,
		Text:           input.Text,
		Classification: input.Classification,
		Comment:        input.Comment,
		Modifications:  input.Modifications,
		Active:         input.Active,
		TextUpdatable:  false,
	}

	result := db.DB.
		Where(&models.Obligation{Topic: obligation.Topic}).
		Or(&models.Obligation{Md5: obligation.Md5}).
		FirstOrCreate(&obligation)

	if result.RowsAffected == 0 {
		er := models.LicenseError{
			Status:  http.StatusConflict,
			Message: "can not create obligation with same topic or text",
			Error: fmt.Sprintf("Error: Obligation with topic '%s' or Text '%s'... already exists",
				obligation.Topic, obligation.Text[0:10]),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusConflict, er)
		return
	}
	if result.Error != nil {
		er := models.LicenseError{
			Status:    http.StatusInternalServerError,
			Message:   "Failed to create obligation",
			Error:     result.Error.Error(),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusInternalServerError, er)
		return
	}
	for _, i := range input.Shortnames {
		var license models.LicenseDB
		db.DB.Where(models.LicenseDB{Shortname: i}).Find(&license)
		obmap := models.ObligationMap{
			ObligationPk: obligation.Id,
			RfPk:         license.Id,
		}
		db.DB.Create(&obmap)
	}

	res := models.ObligationResponse{
		Data:   []models.Obligation{obligation},
		Status: http.StatusCreated,
		Meta: &models.PaginationMeta{
			ResourceCount: 1,
		},
	}

	c.JSON(http.StatusCreated, res)
}

// UpdateObligation updates an existing active obligation record
//
//	@Summary		Update obligation
//	@Description	Update an existing obligation record
//	@Id				UpdateObligation
//	@Tags			Obligations
//	@Accept			json
//	@Produce		json
//	@Param			topic		path		string									true	"Topic of the obligation to be updated"
//	@Param			obligation	body		models.ObligationPATCHRequestJSONSchema	true	"Obligation to be updated"
//	@Success		200			{object}	models.ObligationResponse
//	@Failure		400			{object}	models.LicenseError	"Invalid request"
//	@Failure		404			{object}	models.LicenseError	"No obligation with given topic found"
//	@Failure		500			{object}	models.LicenseError	"Unable to update obligation"
//	@Security		ApiKeyAuth
//	@Router			/obligations/{topic} [patch]
func UpdateObligation(c *gin.Context) {
	_ = db.DB.Transaction(func(tx *gorm.DB) error {
		var updates models.ObligationPATCHRequestJSONSchema
		var oldObligation models.Obligation
		newObligationMap := make(map[string]interface{})

		username := c.GetString("username")
		tp := c.Param("topic")
		if err := tx.Model(&oldObligation).Where(models.Obligation{Topic: tp}).First(&oldObligation).Error; err != nil {
			er := models.LicenseError{
				Status:    http.StatusNotFound,
				Message:   fmt.Sprintf("obligation with topic '%s' not found", tp),
				Error:     err.Error(),
				Path:      c.Request.URL.Path,
				Timestamp: time.Now().Format(time.RFC3339),
			}
			c.JSON(http.StatusNotFound, er)
			return err
		}

		if err := c.ShouldBindJSON(&updates); err != nil {
			er := models.LicenseError{
				Status:    http.StatusBadRequest,
				Message:   "invalid json body",
				Error:     err.Error(),
				Path:      c.Request.URL.Path,
				Timestamp: time.Now().Format(time.RFC3339),
			}
			c.JSON(http.StatusBadRequest, er)
			return err
		}

		if updates.Text.Value != "" && !oldObligation.TextUpdatable && updates.Text.Value != oldObligation.Text {
			er := models.LicenseError{
				Status:    http.StatusBadRequest,
				Message:   "Can not update obligation text",
				Error:     "invalid request",
				Path:      c.Request.URL.Path,
				Timestamp: time.Now().Format(time.RFC3339),
			}
			c.JSON(http.StatusBadRequest, er)
			return errors.New("invalid request")
		}

		if oldObligation.TextUpdatable && (updates.Text.Value != "" && updates.Text.Value != oldObligation.Text) {
			updatedHash := md5.Sum([]byte(updates.Text.Value))
			updatedMd5hash := hex.EncodeToString(updatedHash[:])
			newObligationMap["md5"] = updatedMd5hash
			newObligationMap["text"] = updates.Text.Value
		}

		if updates.Type.IsNotUndefined {
			if updates.Type.Value == "" {
				er := models.LicenseError{
					Status:    http.StatusBadRequest,
					Message:   "Type cannot be an empty string",
					Error:     "invalid request",
					Path:      c.Request.URL.Path,
					Timestamp: time.Now().Format(time.RFC3339),
				}
				c.JSON(http.StatusBadRequest, er)
				return errors.New("invalid request")
			}
			newObligationMap["type"] = updates.Type.Value
		}

		if updates.Classification.IsNotUndefined {
			if updates.Classification.Value == "" {
				er := models.LicenseError{
					Status:    http.StatusBadRequest,
					Message:   "Classification cannot be an empty string",
					Error:     "invalid request",
					Path:      c.Request.URL.Path,
					Timestamp: time.Now().Format(time.RFC3339),
				}
				c.JSON(http.StatusBadRequest, er)
				return errors.New("invalid request")
			}
			newObligationMap["classification"] = updates.Classification.Value
		}

		if updates.Modifications.IsNotUndefined {
			newObligationMap["modifications"] = updates.Modifications.Value
		}

		if updates.Comment.IsNotUndefined {
			var comment models.NullString
			if !updates.Comment.IsNull {
				comment.Valid = true
				comment.String = updates.Comment.Value
			}
			newObligationMap["comment"] = comment
		}

		if updates.Active.IsNotUndefined {
			newObligationMap["active"] = updates.Active.Value
		}

		if updates.TextUpdatable.IsNotUndefined {
			newObligationMap["text_updatable"] = updates.TextUpdatable.Value
		}

		var newObligation models.Obligation
		newObligation.Id = oldObligation.Id
		if err := tx.Model(&newObligation).Clauses(clause.Returning{}).Updates(newObligationMap).Error; err != nil {
			er := models.LicenseError{
				Status:    http.StatusInternalServerError,
				Message:   "Failed to update license",
				Error:     err.Error(),
				Path:      c.Request.URL.Path,
				Timestamp: time.Now().Format(time.RFC3339),
			}
			c.JSON(http.StatusInternalServerError, er)
			return err
		}

		var user models.User
		if err := tx.Where(models.User{Username: username}).First(&user).Error; err != nil {
			er := models.LicenseError{
				Status:    http.StatusInternalServerError,
				Message:   "Failed to update license",
				Error:     err.Error(),
				Path:      c.Request.URL.Path,
				Timestamp: time.Now().Format(time.RFC3339),
			}
			c.JSON(http.StatusInternalServerError, er)
			return err
		}

		var changes []models.ChangeLog

		if oldObligation.Topic != newObligation.Topic {
			changes = append(changes, models.ChangeLog{
				Field:        "Topic",
				OldValue:     &oldObligation.Topic,
				UpdatedValue: &newObligation.Topic,
			})
		}
		if oldObligation.Type != newObligation.Type {
			changes = append(changes, models.ChangeLog{
				Field:        "Type",
				OldValue:     &oldObligation.Type,
				UpdatedValue: &newObligation.Type,
			})
		}
		if oldObligation.Text != newObligation.Text {
			changes = append(changes, models.ChangeLog{
				Field:        "Text",
				OldValue:     &oldObligation.Text,
				UpdatedValue: &newObligation.Text,
			})
		}
		if oldObligation.Classification != newObligation.Classification {
			oldVal := strconv.FormatBool(oldObligation.Modifications)
			newVal := strconv.FormatBool(newObligation.Modifications)
			changes = append(changes, models.ChangeLog{
				Field:        "Classification",
				OldValue:     &oldVal,
				UpdatedValue: &newVal,
			})
		}
		if oldObligation.Modifications != newObligation.Modifications {
			oldVal := strconv.FormatBool(oldObligation.Modifications)
			newVal := strconv.FormatBool(newObligation.Modifications)
			changes = append(changes, models.ChangeLog{
				Field:        "Modifications",
				OldValue:     &oldVal,
				UpdatedValue: &newVal,
			})
		}
		if oldObligation.Comment != newObligation.Comment {
			var oldVal, newVal *string
			if oldObligation.Comment.Valid {
				oldVal = &oldObligation.Comment.String
			}
			if newObligation.Comment.Valid {
				newVal = &newObligation.Comment.String
			}
			changes = append(changes, models.ChangeLog{
				Field:        "Comment",
				OldValue:     oldVal,
				UpdatedValue: newVal,
			})
		}
		if oldObligation.Active != newObligation.Active {
			oldVal := strconv.FormatBool(oldObligation.Active)
			newVal := strconv.FormatBool(newObligation.Active)
			changes = append(changes, models.ChangeLog{
				Field:        "Active",
				OldValue:     &oldVal,
				UpdatedValue: &newVal,
			})
		}
		if oldObligation.TextUpdatable != newObligation.TextUpdatable {
			oldVal := strconv.FormatBool(oldObligation.TextUpdatable)
			newVal := strconv.FormatBool(newObligation.TextUpdatable)
			changes = append(changes, models.ChangeLog{
				Field:        "TextUpdatable",
				OldValue:     &oldVal,
				UpdatedValue: &newVal,
			})
		}

		if len(changes) != 0 {
			audit := models.Audit{
				UserId:     user.Id,
				TypeId:     newObligation.Id,
				Timestamp:  time.Now(),
				Type:       "Obligation",
				ChangeLogs: changes,
			}

			if err := tx.Create(&audit).Error; err != nil {
				er := models.LicenseError{
					Status:    http.StatusInternalServerError,
					Message:   "Failed to update license",
					Error:     err.Error(),
					Path:      c.Request.URL.Path,
					Timestamp: time.Now().Format(time.RFC3339),
				}
				c.JSON(http.StatusInternalServerError, er)
				return err
			}
		}

		res := models.ObligationResponse{
			Data:   []models.Obligation{newObligation},
			Status: http.StatusOK,
			Meta: &models.PaginationMeta{
				ResourceCount: 1,
			},
		}
		c.JSON(http.StatusOK, res)

		return nil
	})
}

// DeleteObligation marks an existing obligation record as inactive
//
//	@Summary		Deactivate obligation
//	@Description	Deactivate an obligation
//	@Id				DeleteObligation
//	@Tags			Obligations
//	@Accept			json
//	@Produce		json
//	@Param			topic	path	string	true	"Topic of the obligation to be updated"
//	@Success		204
//	@Failure		404	{object}	models.LicenseError	"No obligation with given topic found"
//	@Security		ApiKeyAuth
//	@Router			/obligations/{topic} [delete]
func DeleteObligation(c *gin.Context) {
	var obligation models.Obligation
	tp := c.Param("topic")
	if err := db.DB.Where(models.Obligation{Topic: tp}).First(&obligation).Error; err != nil {
		er := models.LicenseError{
			Status:    http.StatusNotFound,
			Message:   fmt.Sprintf("obligation with topic '%s' not found", tp),
			Error:     err.Error(),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusNotFound, er)
		return
	}
	obligation.Active = false
	db.DB.Where(models.Obligation{Topic: tp}).Save(&obligation)
	c.Status(http.StatusNoContent)
}

// GetObligationAudits fetches audits corresponding to an obligation

// @Summary		Fetches audits corresponding to an obligation
// @Description	Fetches audits corresponding to an obligation
// @Id				GetObligationAudits
// @Tags			Obligations
// @Accept			json
// @Produce		json
// @Param			topic	path		string	true	"Topic of the obligation for which audits need to be fetched"
// @Param			page	query		int		false	"Page number"
// @Param			limit	query		int		false	"Number of records per page"
// @Success		200		{object}	models.AuditResponse
// @Failure		404		{object}	models.LicenseError	"No obligation with given topic found"
// @Failure		500		{object}	models.LicenseError	"unable to find audits with such obligation topic"
// @Security		ApiKeyAuth
// @Router			/obligations/{topic}/audits [get]
func GetObligationAudits(c *gin.Context) {
	var obligation models.Obligation
	topic := c.Param("topic")

	result := db.DB.Where(models.Obligation{Topic: topic}).Select("id").First(&obligation)
	if result.Error != nil {
		er := models.LicenseError{
			Status:    http.StatusNotFound,
			Message:   fmt.Sprintf("obligation with topic '%s' not found", topic),
			Error:     result.Error.Error(),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusNotFound, er)
		return
	}

	var audits []models.Audit
	query := db.DB.Model(&models.Audit{})
	query.Where(models.Audit{TypeId: obligation.Id, Type: "Obligation"})
	_ = utils.PreparePaginateResponse(c, query, &models.AuditResponse{})

	res := query.Find(&audits)
	if res.Error != nil {
		er := models.LicenseError{
			Status:    http.StatusInternalServerError,
			Message:   "unable to find audits with such obligation topic",
			Error:     res.Error.Error(),
			Path:      c.Request.URL.Path,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		c.JSON(http.StatusInternalServerError, er)
		return
	}

	response := models.AuditResponse{
		Data:   audits,
		Status: http.StatusOK,
		Meta: &models.PaginationMeta{
			ResourceCount: len(audits),
		},
	}

	c.JSON(http.StatusOK, response)
}
