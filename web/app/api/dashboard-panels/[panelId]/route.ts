import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, requestJSON, sameOriginMutation } from "@/lib/server-api";

export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ panelId: string }> };

export async function PATCH(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const { panelId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(panelId)) return errorResponse(400, "invalid_panel_id", "The panel identifier is invalid.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object" || Array.isArray(body)) {
    return errorResponse(400, "invalid_request", "A panel configuration is required.");
  }
  return backendResponse(await callBackend("/v1/dashboard-panels/" + encodeURIComponent(panelId), {
    method: "PATCH",
    body,
  }));
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const { panelId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(panelId)) return errorResponse(400, "invalid_panel_id", "The panel identifier is invalid.");
  return backendResponse(await callBackend("/v1/dashboard-panels/" + encodeURIComponent(panelId), { method: "DELETE" }));
}
