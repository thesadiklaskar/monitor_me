package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"monitor-me.internal/backend/internal/identity"
)

func (api *API) listProjects(writer http.ResponseWriter, request *http.Request) {
	projects, err := api.identities.ListProjects(request.Context(), principalFromRequest(request))
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "project_list_failed", "Projects could not be loaded.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"projects": projects})
}

func (api *API) createProject(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	project, err := api.identities.CreateProject(request.Context(), principalFromRequest(request), input.Name, input.Slug)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can create projects.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusConflict, "invalid_project", "Project name or slug is invalid or already in use.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "project_create_failed", "The project could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{"project": project})
	}
}

func (api *API) deleteProject(writer http.ResponseWriter, request *http.Request) {
	projectID := strings.TrimSpace(request.PathValue("projectID"))
	err := api.identities.DeleteProject(request.Context(), principalFromRequest(request), projectID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Only organization owners and administrators can delete projects.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The project was not found in this organization.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "project_delete_failed", "The project could not be deleted.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}

func (api *API) listServerProjectMappings(writer http.ResponseWriter, request *http.Request) {
	serverID := strings.TrimSpace(request.PathValue("serverID"))
	mappings, err := api.identities.ListServerProjectMappings(request.Context(), principalFromRequest(request), serverID)
	switch {
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The server was not found in this organization.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "mapping_list_failed", "Server workload mappings could not be loaded.")
	default:
		writeJSON(writer, http.StatusOK, map[string]any{"mappings": mappings})
	}
}

func (api *API) createServerProjectMapping(writer http.ResponseWriter, request *http.Request) {
	serverID := strings.TrimSpace(request.PathValue("serverID"))
	var input struct {
		ProjectID         string `json:"project_id"`
		WorkloadType      string `json:"workload_type"`
		WorkloadSelector  string `json:"workload_selector"`
		AttributionMethod string `json:"attribution_method"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	mapping, err := api.identities.CreateServerProjectMapping(
		request.Context(), principalFromRequest(request), serverID, input.ProjectID,
		input.WorkloadType, input.WorkloadSelector, input.AttributionMethod,
	)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Your organization role cannot manage workload mappings.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The server or project was not found in this organization.")
	case errors.Is(err, identity.ErrInvalidInput):
		writeAPIError(writer, http.StatusConflict, "invalid_mapping", "The mapping is invalid or that workload selector is already assigned.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "mapping_create_failed", "The workload mapping could not be created.")
	default:
		writeJSON(writer, http.StatusCreated, map[string]any{"mapping": mapping})
	}
}

func (api *API) deleteServerProjectMapping(writer http.ResponseWriter, request *http.Request) {
	mappingID := strings.TrimSpace(request.PathValue("mappingID"))
	err := api.identities.DeleteServerProjectMapping(request.Context(), principalFromRequest(request), mappingID)
	switch {
	case errors.Is(err, identity.ErrForbidden):
		writeAPIError(writer, http.StatusForbidden, "forbidden", "Your organization role cannot remove workload mappings.")
	case errors.Is(err, identity.ErrNotFound):
		writeAPIError(writer, http.StatusNotFound, "not_found", "The workload mapping was not found in this organization.")
	case err != nil:
		writeAPIError(writer, http.StatusInternalServerError, "mapping_delete_failed", "The workload mapping could not be removed.")
	default:
		writer.WriteHeader(http.StatusNoContent)
	}
}
