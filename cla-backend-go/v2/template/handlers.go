// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package template

import (
	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	v1Events "github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/template"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	v1Template "github.com/communitybridge/easycla/cla-backend-go/template"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

// Configure API call
func Configure(api *operations.EasyclaAPI, service v1Template.Service, eventsService v1Events.Service) {
	// Retrieve a list of available templates
	api.TemplateGetTemplatesHandler = template.GetTemplatesHandlerFunc(func(params template.GetTemplatesParams, user *auth.User) middleware.Responder {

		templates, err := service.GetTemplates(params.HTTPRequest.Context())
		if err != nil {
			return template.NewGetTemplatesBadRequest().WithPayload(errorResponse(err))
		}
		return template.NewGetTemplatesOK().WithPayload(templates)
	})

	api.TemplateCreateCLAGroupTemplateHandler = template.CreateCLAGroupTemplateHandlerFunc(func(params template.CreateCLAGroupTemplateParams, user *auth.User) middleware.Responder {
		pdfUrls, err := service.CreateCLAGroupTemplate(params.HTTPRequest.Context(), params.ClaGroupID, &params.Body)
		utils.SetAuthUserProperties(user, params.XUSERNAME, params.XEMAIL)
		if err != nil {
			log.Warnf("Error generating PDFs from provided templates, error: %v", err)
			return template.NewGetTemplatesBadRequest().WithPayload(errorResponse(err))
		}
		eventsService.LogEvent(&events.LogEventArgs{
			EventType:  events.CLATemplateCreated,
			ProjectID:  params.ClaGroupID,
			LfUsername: user.UserName,
			EventData:  &events.CLATemplateCreatedEventData{},
		})

		return template.NewCreateCLAGroupTemplateOK().WithPayload(pdfUrls)
	})
}

type codedResponse interface {
	Code() string
}

func errorResponse(err error) *models.ErrorResponse {
	code := ""
	if e, ok := err.(codedResponse); ok {
		code = e.Code()
	}

	e := models.ErrorResponse{
		Code:    code,
		Message: err.Error(),
	}

	return &e
}
