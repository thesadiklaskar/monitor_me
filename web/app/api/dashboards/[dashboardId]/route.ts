import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, requestJSON, sameOriginMutation } from "@/lib/server-api";

export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ dashboardId: string }> };

export async function PATCH(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const { dashboardId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(dashboardId)) return errorResponse(400, "invalid_dashboard_id", "The dashboard identifier is invalid.");
  const body = await requestJSON(request);
  if (!body || typeof body !== "object" || Array.isArray(body)) {
    return errorResponse(400, "invalid_request", "Dashboard data is required.");
  }
  return backendResponse(await callBackend("/v1/dashboards/" + encodeURIComponent(dashboardId), { method: "PATCH", body }));
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const { dashboardId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(dashboardId)) return errorResponse(400, "invalid_dashboard_id", "The dashboard identifier is invalid.");
  return backendResponse(await callBackend("/v1/dashboards/" + encodeURIComponent(dashboardId), { method: "DELETE" }));
}
