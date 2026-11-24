import { NextRequest } from "next/server";
import { backendResponse, callBackend, errorResponse, sameOriginMutation } from "@/lib/server-api";

export const dynamic = "force-dynamic";
type RouteContext = { params: Promise<{ dashboardId: string }> };

export async function DELETE(request: NextRequest, context: RouteContext) {
  if (!sameOriginMutation(request)) return errorResponse(403, "origin_rejected", "The request origin is not allowed.");
  const { dashboardId } = await context.params;
  if (!/^[0-9a-f-]{36}$/i.test(dashboardId)) return errorResponse(400, "invalid_dashboard_id", "The dashboard identifier is invalid.");
  return backendResponse(await callBackend("/v1/dashboards/" + encodeURIComponent(dashboardId), { method: "DELETE" }));
}
