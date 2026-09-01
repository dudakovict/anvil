package rest

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/dudakovict/anvil/internal/core/domain"
	"github.com/dudakovict/anvil/internal/core/port"
	"github.com/dudakovict/anvil/internal/platform/auth"
	"github.com/dudakovict/anvil/internal/platform/query"
	"github.com/dudakovict/anvil/internal/platform/validate"
	"github.com/dudakovict/anvil/internal/platform/web"
)

type userHandler struct {
	svc port.UserService
}

func (h *userHandler) publicRoutes(r chi.Router) {
	r.Post("/users", web.Handle(h.create))
}

func (h *userHandler) protectedRoutes(r chi.Router) {
	r.Get("/me", web.Handle(h.me))
	r.Get("/users/{id}", web.Handle(h.get))
	r.Put("/users/{id}", web.Handle(h.update))
	r.Delete("/users/{id}", web.Handle(h.delete))
}

func (h *userHandler) adminRoutes(r chi.Router) {
	r.Get("/users", web.Handle(h.list))
}

// create godoc
//
//	@Summary	Create a user
//	@Tags		users
//	@Accept		json
//	@Produce	json
//	@Param		request	body		rest.CreateUserRequest	true	"user payload"
//	@Success	201		{object}	rest.UserResponse
//	@Failure	400		{object}	web.ErrorResponse
//	@Failure	409		{object}	web.ErrorResponse
//	@Failure	500		{object}	web.ErrorResponse
//	@Router		/users [post]
func (h *userHandler) create(ctx context.Context, r *http.Request) web.Response {
	req, err := web.Decode[CreateUserRequest](r)
	if err != nil {
		return decodeError(ctx, err)
	}

	u, err := h.svc.Create(ctx, req.Email, req.Name, req.Password)
	if err != nil {
		return errorResponse(ctx, err)
	}

	return web.Respond(ctx, http.StatusCreated, toUserResponse(u))
}

// me godoc
//
//	@Summary	Get the authenticated user
//	@Tags		users
//	@Security	BearerAuth
//	@Produce	json
//	@Success	200	{object}	rest.UserResponse
//	@Failure	401	{object}	web.ErrorResponse
//	@Failure	404	{object}	web.ErrorResponse
//	@Failure	500	{object}	web.ErrorResponse
//	@Router		/me [get]
func (h *userHandler) me(ctx context.Context, _ *http.Request) web.Response {
	claims, ok := auth.FromContext(ctx)
	if !ok {
		return web.RespondError(ctx, http.StatusUnauthorized, "unauthorized")
	}

	u, err := h.svc.Get(ctx, claims.UserID)
	if err != nil {
		return errorResponse(ctx, err)
	}

	return web.Respond(ctx, http.StatusOK, toUserResponse(u))
}

// get godoc
//
//	@Summary	Get a user by id
//	@Tags		users
//	@Security	BearerAuth
//	@Produce	json
//	@Param		id	path		int	true	"user id"
//	@Success	200	{object}	rest.UserResponse
//	@Failure	400	{object}	web.ErrorResponse
//	@Failure	403	{object}	web.ErrorResponse
//	@Failure	404	{object}	web.ErrorResponse
//	@Failure	500	{object}	web.ErrorResponse
//	@Router		/users/{id} [get]
func (h *userHandler) get(ctx context.Context, r *http.Request) web.Response {
	id, err := idParam(r)
	if err != nil {
		return web.RespondError(ctx, http.StatusBadRequest, "invalid user id")
	}

	if !canAccess(ctx, id) {
		return errorResponse(ctx, domain.ErrForbidden)
	}

	u, err := h.svc.Get(ctx, id)
	if err != nil {
		return errorResponse(ctx, err)
	}

	return web.Respond(ctx, http.StatusOK, toUserResponse(u))
}

// userSortFields whitelists ?sort= values, mapping API names to SQL columns.
var userSortFields = map[string]string{
	"id":         "id",
	"email":      "email",
	"name":       "name",
	"created_at": "created_at",
	"updated_at": "updated_at",
}

// list godoc
//
//	@Summary	List users
//	@Tags		users
//	@Security	BearerAuth
//	@Produce	json
//	@Param		limit	query		int		false	"page size (default 10, max 100)"
//	@Param		page	query		int		false	"page number, 1-based (default 1)"
//	@Param		sort	query		string	false	"comma-separated sort fields, minus prefix for descending (e.g. -created_at,name)"
//	@Param		email	query		string	false	"filter by email (exact)"
//	@Param		name	query		string	false	"filter by name (contains)"
//	@Success	200		{object}	rest.UsersPage
//	@Failure	400		{object}	web.ErrorResponse
//	@Failure	403		{object}	web.ErrorResponse
//	@Failure	500		{object}	web.ErrorResponse
//	@Router		/users [get]
func (h *userHandler) list(ctx context.Context, r *http.Request) web.Response {
	page, err := query.ParsePage(r)
	if err != nil {
		return web.RespondError(ctx, http.StatusBadRequest, err.Error())
	}

	order, err := query.ParseOrder(r, userSortFields)
	if err != nil {
		return web.RespondError(ctx, http.StatusBadRequest, err.Error())
	}

	q, err := web.DecodeQuery[ListUsersQuery](r)
	if err != nil {
		return decodeError(ctx, err)
	}

	filter := domain.UserFilter{
		Email: q.Email,
		Name:  q.Name,
	}

	users, total, err := h.svc.List(ctx, filter, page, order)
	if err != nil {
		return errorResponse(ctx, err)
	}

	return web.Respond(ctx, http.StatusOK, query.NewResult(toUserResponses(users), total, page))
}

// update godoc
//
//	@Summary	Update a user
//	@Tags		users
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		id		path		int						true	"user id"
//	@Param		request	body		rest.UpdateUserRequest	true	"user payload"
//	@Success	200		{object}	rest.UserResponse
//	@Failure	400		{object}	web.ErrorResponse
//	@Failure	403		{object}	web.ErrorResponse
//	@Failure	404		{object}	web.ErrorResponse
//	@Failure	409		{object}	web.ErrorResponse
//	@Failure	500		{object}	web.ErrorResponse
//	@Router		/users/{id} [put]
func (h *userHandler) update(ctx context.Context, r *http.Request) web.Response {
	id, err := idParam(r)
	if err != nil {
		return web.RespondError(ctx, http.StatusBadRequest, "invalid user id")
	}

	if !canAccess(ctx, id) {
		return errorResponse(ctx, domain.ErrForbidden)
	}

	req, err := web.Decode[UpdateUserRequest](r)
	if err != nil {
		return decodeError(ctx, err)
	}

	u, err := h.svc.Update(ctx, id, req.Email, req.Name)
	if err != nil {
		return errorResponse(ctx, err)
	}

	return web.Respond(ctx, http.StatusOK, toUserResponse(u))
}

// delete godoc
//
//	@Summary	Delete a user
//	@Tags		users
//	@Security	BearerAuth
//	@Param		id	path	int	true	"user id"
//	@Success	204
//	@Failure	400	{object}	web.ErrorResponse
//	@Failure	403	{object}	web.ErrorResponse
//	@Failure	404	{object}	web.ErrorResponse
//	@Failure	500	{object}	web.ErrorResponse
//	@Router		/users/{id} [delete]
func (h *userHandler) delete(ctx context.Context, r *http.Request) web.Response {
	id, err := idParam(r)
	if err != nil {
		return web.RespondError(ctx, http.StatusBadRequest, "invalid user id")
	}

	if !canAccess(ctx, id) {
		return errorResponse(ctx, domain.ErrForbidden)
	}

	if err := h.svc.Delete(ctx, id); err != nil {
		return errorResponse(ctx, err)
	}

	return web.Respond(ctx, http.StatusNoContent, nil)
}

func idParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// canAccess allows admins and the owner of the targeted account.
func canAccess(ctx context.Context, id int64) bool {
	claims, ok := auth.FromContext(ctx)

	return ok && (claims.Role == string(domain.RoleAdmin) || claims.UserID == id)
}

func decodeError(ctx context.Context, err error) web.Response {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return web.RespondError(ctx, http.StatusRequestEntityTooLarge, "request body too large")
	}

	if fields, ok := errors.AsType[validate.FieldErrors](err); ok {
		return web.Respond(ctx, http.StatusBadRequest, web.ErrorResponse{
			Error:  "validation failed",
			Fields: fields,
		})
	}

	return web.RespondError(ctx, http.StatusBadRequest, "invalid request body")
}

func errorResponse(ctx context.Context, err error) web.Response {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return web.RespondError(ctx, http.StatusNotFound, "user not found")
	case errors.Is(err, domain.ErrInvalidUser):
		return web.RespondError(ctx, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		return web.RespondError(ctx, http.StatusConflict, "email already taken")
	case errors.Is(err, domain.ErrInvalidCredentials):
		return web.RespondError(ctx, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, domain.ErrForbidden):
		return web.RespondError(ctx, http.StatusForbidden, "forbidden")
	default:
		return web.RespondError(ctx, http.StatusInternalServerError, "internal server error", err)
	}
}
