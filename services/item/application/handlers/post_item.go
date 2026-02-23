package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/ghuser/ghproject/pkg/errhttp"
	"github.com/ghuser/ghproject/pkg/httpx"
	pkgvalidator "github.com/ghuser/ghproject/pkg/validator"
	appsvcs "github.com/ghuser/ghproject/services/item/application/services"
)

// CreateItemRequest is the request body for POST /item.
type CreateItemRequest struct {
	Name      string `json:"name" validate:"required,min=3,max=255" example:"Sample Item"`
	OwnerName string `json:"owner_name" validate:"required,min=3,max=255" example:"Sample Item"`
} // @name CreateItemRequest

// CreateItemResponse is returned on successful item creation.
type CreateItemResponse struct {
	ID        uuid.UUID `json:"id"         example:"123e4567-e89b-12d3-a456-426614174000"`
	OrgID     uuid.UUID `json:"org_id"     example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string    `json:"name"       example:"Sample Item"`
	CreatedAt time.Time `json:"created_at" example:"2024-01-15T10:30:00Z"`
} // @name CreateItemResponse

// ErrorResponse is returned on all error responses.
type ErrorResponse struct {
	Error string `json:"error" example:"org_id and name are required"`
} // @name ErrorResponse

// PostItemHandler handles POST /item requests.
type PostItemHandler struct {
	svc *appsvcs.Services
}

// NewPostItemHandler returns a PostItemHandler backed by the given services.
func NewPostItemHandler(svc *appsvcs.Services) *PostItemHandler {
	return &PostItemHandler{svc: svc}
}

// Execute creates a new item.
//
//	@Summary		Create item
//	@Description	Creates a new item scoped to an organization
//	@Tags			items
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateItemRequest	true	"Item creation request"
//	@Success		201		{object}	CreateItemResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		422		{object}	ErrorResponse
//	@Router			/item [post]
func (h *PostItemHandler) Execute(w http.ResponseWriter, r *http.Request) {
	//orgID, err := auth.OrgIDFromCtx(r.Context())
	//if err != nil {
	//	httpx.JSON(w, http.StatusUnauthorized, ErrorResponse{Error: "authentication required"})
	//	return
	//}

	req, ok := pkgvalidator.ValidateRequest[CreateItemRequest](w, r)
	if !ok {
		return
	}

	item, err := h.svc.Item.Create(r.Context(), uuid.New(), req.Name)
	if err != nil {
		errhttp.WriteError(w, err)
		return
	}

	httpx.JSON(w, http.StatusCreated, CreateItemResponse{
		ID:        item.ID,
		OrgID:     item.OrgID,
		Name:      item.Name.String(),
		CreatedAt: item.CreatedAt,
	})
}
