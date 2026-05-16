// skills.go — PLACEHOLDER entity file (Plan 03-05 scaffold).
//
// Plan 03-07 (Wave 4) REPLACES the bodies of all 5 methods below with
// the real implementations (CAT-02, CAT-07, CAT-08, CAT-09, CAT-10).
package catalog

import (
	"context"

	"github.com/luongdev/open-routing/services/api/internal/api"
)

// ListSkills — PLACEHOLDER replaced by Plan 03-07 (CAT-02, CAT-09, CAT-10).
func (h *Handlers) ListSkills(_ context.Context, _ api.ListSkillsRequestObject) (api.ListSkillsResponseObject, error) {
	return api.ListSkills500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// CreateSkill — PLACEHOLDER replaced by Plan 03-07 (CAT-02).
func (h *Handlers) CreateSkill(_ context.Context, _ api.CreateSkillRequestObject) (api.CreateSkillResponseObject, error) {
	return api.CreateSkill500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// DeleteSkill — PLACEHOLDER replaced by Plan 03-07 (CAT-02, CAT-09).
func (h *Handlers) DeleteSkill(_ context.Context, _ api.DeleteSkillRequestObject) (api.DeleteSkillResponseObject, error) {
	return api.DeleteSkill500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// GetSkill — PLACEHOLDER replaced by Plan 03-07 (CAT-02, CAT-11).
func (h *Handlers) GetSkill(_ context.Context, _ api.GetSkillRequestObject) (api.GetSkillResponseObject, error) {
	return api.GetSkill500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}

// UpdateSkill — PLACEHOLDER replaced by Plan 03-07 (CAT-02, CAT-08).
func (h *Handlers) UpdateSkill(_ context.Context, _ api.UpdateSkillRequestObject) (api.UpdateSkillResponseObject, error) {
	return api.UpdateSkill500JSONResponse{InternalServerErrorJSONResponse: notImplementedBody()}, nil
}
