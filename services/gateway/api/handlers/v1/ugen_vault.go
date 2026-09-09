package v1

import (
	"errors"
	"sort"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

type CreateVaultKeyRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type UpdateVaultKeyRequest struct {
	Value string `json:"value"`
}

type VaultKeyEntry struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type GetListVaultKeysResponse struct {
	Data  []VaultKeyEntry `json:"data"`
	Count int             `json:"count"`
}

func (h *HandlerV1) getResourceEnvVaultPath(c *gin.Context, projectId, environmentId string) (string, error) {
	resEnv, err := h.companyServices.Resource().GetDefaultResourceEnvironment(
		c.Request.Context(),
		&pb.GetDefaultResourceEnvironmentReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId,
		},
	)
	if err != nil {
		return "", err
	}
	if resEnv.GetVaultPath() == "" {
		return "", errors.New("resource environment has no vault path configured")
	}
	return resEnv.GetVaultPath(), nil
}

// GetListVaultKeys godoc
// @Security ApiKeyAuth
// @ID get_list_vault_keys
// @Router /v1/vault/keys [GET]
// @Summary Get list of vault keys
// @Description Returns paginated key-value pairs stored at the default resource environment's vault path for the current project and environment
// @Tags Vault
// @Accept json
// @Produce json
// @Param offset query int false "offset"
// @Param limit query int false "limit"
// @Success 200 {object} status_http.Response{data=GetListVaultKeysResponse} "Vault keys"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListVaultKeys(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, errors.New("error getting environment id | not valid"))
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	vaultPath, err := h.getResourceEnvVaultPath(c, projectId.(string), environmentId.(string))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if h.vault == nil {
		h.HandleResponse(c, status_http.InternalServerError, "vault client is not configured")
		return
	}

	secrets, err := h.vault.Get(c.Request.Context(), vaultPath)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		if k == "database" || k == "host" || k == "password" || k == "port" || k == "username" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	total := len(keys)

	if offset >= total {
		h.HandleResponse(c, status_http.OK, GetListVaultKeysResponse{Data: []VaultKeyEntry{}, Count: total})
		return
	}

	end := offset + limit
	if end > total {
		end = total
	}

	entries := make([]VaultKeyEntry, 0, end-offset)
	for _, k := range keys[offset:end] {
		entries = append(entries, VaultKeyEntry{Key: k, Value: ""})
	}

	h.HandleResponse(c, status_http.OK, GetListVaultKeysResponse{Data: entries, Count: total})
}

// CreateVaultKey godoc
// @Security ApiKeyAuth
// @ID create_vault_key
// @Router /v1/vault/keys [POST]
// @Summary Create or update a vault key
// @Description Adds a key-value pair to the secret stored at the default resource environment's vault path
// @Tags Vault
// @Accept json
// @Produce json
// @Param body body CreateVaultKeyRequest true "CreateVaultKeyRequest"
// @Success 200 {object} status_http.Response{data=string} "OK"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateVaultKey(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, errors.New("error getting environment id | not valid"))
		return
	}

	var req CreateVaultKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if req.Key == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "key is required")
		return
	}

	vaultPath, err := h.getResourceEnvVaultPath(c, projectId.(string), environmentId.(string))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if h.vault == nil {
		h.HandleResponse(c, status_http.InternalServerError, "vault client is not configured")
		return
	}

	existing, err := h.vault.Get(c.Request.Context(), vaultPath)
	if err != nil {
		existing = map[string]any{}
	}

	existing[req.Key] = req.Value

	if err := h.vault.Put(c.Request.Context(), vaultPath, existing); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, "vault key created successfully")
}

// DeleteVaultKey godoc
// @Security ApiKeyAuth
// @ID delete_vault_key
// @Router /v1/vault/keys/{key} [DELETE]
// @Summary Delete a vault key
// @Description Removes a key-value pair from the secret stored at the default resource environment's vault path
// @Tags Vault
// @Accept json
// @Produce json
// @Param key path string true "key name to delete"
// @Success 200 {object} status_http.Response{data=string} "OK"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteVaultKey(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, errors.New("error getting environment id | not valid"))
		return
	}

	keyName := c.Param("key")
	if keyName == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "key is required")
		return
	}

	vaultPath, err := h.getResourceEnvVaultPath(c, projectId.(string), environmentId.(string))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if h.vault == nil {
		h.HandleResponse(c, status_http.InternalServerError, "vault client is not configured")
		return
	}

	existing, err := h.vault.Get(c.Request.Context(), vaultPath)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if _, exists := existing[keyName]; !exists {
		h.HandleResponse(c, status_http.InvalidArgument, "key not found")
		return
	}

	delete(existing, keyName)

	if err := h.vault.Put(c.Request.Context(), vaultPath, existing); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, "vault key deleted successfully")
}

// UpdateVaultKey godoc
// @Security ApiKeyAuth
// @ID update_vault_key
// @Router /v1/vault/keys/{key} [PUT]
// @Summary Update a vault key
// @Description Updates the value of an existing key in the secret stored at the default resource environment's vault path
// @Tags Vault
// @Accept json
// @Produce json
// @Param key path string true "key name to update"
// @Param body body UpdateVaultKeyRequest true "UpdateVaultKeyRequest"
// @Success 200 {object} status_http.Response{data=string} "OK"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateVaultKey(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, errors.New("error getting environment id | not valid"))
		return
	}

	keyName := c.Param("key")
	if keyName == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "key is required")
		return
	}

	var req UpdateVaultKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	vaultPath, err := h.getResourceEnvVaultPath(c, projectId.(string), environmentId.(string))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if h.vault == nil {
		h.HandleResponse(c, status_http.InternalServerError, "vault client is not configured")
		return
	}

	existing, err := h.vault.Get(c.Request.Context(), vaultPath)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if _, exists := existing[keyName]; !exists {
		h.HandleResponse(c, status_http.InvalidArgument, "key not found")
		return
	}

	existing[keyName] = req.Value

	if err := h.vault.Put(c.Request.Context(), vaultPath, existing); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, "vault key updated successfully")
}
